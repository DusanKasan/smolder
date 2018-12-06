package smolder

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

type register struct {
	// TODO map[return type][key type]func
	resolvers map[reflect.Type]map[reflect.Type]func(context.Context, *loader, interface{}) (interface{}, error)
}

func New() *register {
	m := map[reflect.Type]map[reflect.Type]func(context.Context, *loader, interface{}) (interface{}, error){}
	return &register{resolvers: m}
}

// fn for type T must be:
// 		func([]K) (map[K]*T, error)
// 		func(context.Context, []K) (map[K]*T, error)
// 		func(smolder.loader, []K) (map[K]*T, error)
// 		func(context.Context, smolder.loader, []K) (map[K]*T, error)
// 		func([]K) map[K]*T
// 		func(context.Context, []K) map[K]*T
// 		func(loader, []K) map[K]*T
// 		func(context.Context, smolder.loader, []K) map[K]*T
func (l *register) Register(fn interface{}) error {
	var inTransform func(ctx context.Context, loader *loader, ids interface{}) []reflect.Value
	var outTransform func(vals []reflect.Value) (interface{}, error)

	t := reflect.TypeOf(fn)
	if t.Kind() != reflect.Func {
		return errors.New("fn must be a function")
	}

	var keyType reflect.Type

	switch t.NumOut() {
	case 1:
		if t.Out(0).Kind() != reflect.Map {
			return errors.New("fn's first output must be a map")
		}
		keyType = t.Out(0).Key()
		outTransform = func(vals []reflect.Value) (interface{}, error) {
			return vals[0].Interface(), nil
		}
	case 2:
		if t.Out(0).Kind() != reflect.Map {
			return errors.New("fn's first output must be a map")
		}
		keyType = t.Out(0).Key()

		if !t.Out(1).AssignableTo(reflect.TypeOf((*error)(nil)).Elem()) {
			return errors.New("fn's second output must be an error")
		}
		outTransform = func(vals []reflect.Value) (interface{}, error) {
			var err error
			if vals[1].Interface() != nil {
				err = vals[1].Interface().(error)
			}
			return vals[0].Interface(), err
		}
	default:
		return errors.New("fn must have 1 or 2 output variables")
	}

	switch t.NumIn() {
	case 1:
		if t.In(0).Kind() != reflect.Slice || t.In(0).Elem() != keyType {
			return errors.New("fn's first argument's slice elements must be of the same type elements of the return map")
		}
		inTransform = func(ctx context.Context, loader *loader, ids interface{}) []reflect.Value {
			return []reflect.Value{
				reflect.ValueOf(ids),
			}
		}
	case 2:
		if t.In(1).Kind() != reflect.Slice || t.In(1).Elem() != keyType {
			return errors.New("fn's second argument's slice elements must be of the same type elements of the return map")
		}

		if t.In(0).AssignableTo(reflect.TypeOf((*context.Context)(nil)).Elem()) {
			inTransform = func(ctx context.Context, loader *loader, ids interface{}) []reflect.Value {
				return []reflect.Value{
					reflect.ValueOf(ctx),
					reflect.ValueOf(ids),
				}
			}
		} else if t.In(0).AssignableTo(reflect.TypeOf((*Loader)(nil)).Elem()) {
			inTransform = func(ctx context.Context, loader *loader, ids interface{}) []reflect.Value {
				return []reflect.Value{
					reflect.ValueOf(loader),
					reflect.ValueOf(ids),
				}
			}
		} else {
			return errors.New("fn's first argument must be context.Context or Loader")
		}
	case 3:
		if !t.In(0).AssignableTo(reflect.TypeOf((*context.Context)(nil)).Elem()) {
			return errors.New("fn's first argument must be context.Context")
		}

		if !t.In(1).AssignableTo(reflect.TypeOf((*Loader)(nil)).Elem()) {
			return errors.New("fn's second argument must be smolder.loader")
		}

		if t.In(2).Kind() != reflect.Slice || t.In(2).Elem() != keyType {
			return errors.New("fn's third argument's slice elements must be of the same type elements of the return map")
		}
		inTransform = func(ctx context.Context, loader *loader, ids interface{}) []reflect.Value {
			return []reflect.Value{
				reflect.ValueOf(ctx),
				reflect.ValueOf(loader),
				reflect.ValueOf(ids),
			}
		}
	default:
		return errors.New("fn must have 1, 2 or 3 input params")
	}

	typ := t.Out(0).Elem()
	m := l.resolvers[typ]
	if m == nil {
		l.resolvers[typ] = map[reflect.Type]func(context.Context, *loader, interface{}) (interface{}, error){}
	}

	l.resolvers[typ][keyType] = func(ctx context.Context, loader *loader, ids interface{}) (interface{}, error) {
		if reflect.TypeOf(ids).Kind() != reflect.Slice || reflect.TypeOf(ids).Elem() != keyType {
			return nil, fmt.Errorf("invalid ids type, expecting slice of %v, got %v", keyType.String(), reflect.TypeOf(ids).String())
		}

		return outTransform(reflect.ValueOf(fn).Call(inTransform(ctx, loader, ids)))
	}

	return nil
}

func (l *register) Load(ids interface{}, dst interface{}) error {
	// TODO: handle the fact that both ids and dst can be slices or pointers, just slices for now
	keys := reflect.TypeOf(ids)
	if keys.Kind() != reflect.Slice {
		return errors.New("ids must be a slice")
	}

	typ := reflect.TypeOf(dst)
	if typ.Kind() != reflect.Ptr || typ.Elem().Kind() != reflect.Slice {
		return errors.New("dst must be a pointer to a slice")
	}

	target := typ.Elem().Elem()
	resolved, err := l.resolve(ids, target)
	if err != nil {
		return err
	}

	slice := reflect.New(reflect.SliceOf(target)).Elem()
	for _, k := range resolved.MapKeys() {
		val := resolved.MapIndex(k)
		if target.Kind() != reflect.Ptr {
			val = val.Elem()
		}
		slice = reflect.Append(slice, val)
	}

	reflect.ValueOf(dst).Elem().Set(slice)
	return nil
}

type (
	Loader interface {
		Load(ids interface{}, dst interface{})
	}

	loader struct {
		register    register
		invocations []invocation
	}
)

func (l *loader) Load(ids interface{}, dst interface{}) {
	l.invocations = append(l.invocations, invocation{ids, dst})
}

func (l *loader) execute() error {
	// Group the invocations by the type they want to resolve and the type of
	// keys they respond to and group the IDs into distinct groups by the type.
	typeInvocations := map[reflect.Type]map[reflect.Type][]invocation{}
	// map of return type to map of key type to map of existing keys
	typeIds := map[reflect.Type]map[reflect.Type]map[interface{}]bool{}
	for _, inv := range l.invocations {
		typ := reflect.TypeOf(inv.dst).Elem().Elem()
		if typ.Kind() != reflect.Ptr {
			typ = reflect.PtrTo(typ)
		}
		keyType := reflect.TypeOf(inv.ids).Elem()

		if typeInvocations[typ] == nil {
			typeInvocations[typ] = map[reflect.Type][]invocation{}
		}
		if typeIds[typ] == nil {
			typeIds[typ] = map[reflect.Type]map[interface{}]bool{}
		}

		typeInvocations[typ][keyType] = append(typeInvocations[typ][keyType], inv)
		if _, ok := typeIds[typ][keyType]; !ok {
			typeIds[typ][keyType] = map[interface{}]bool{}
		}

		for i := 0; i < reflect.ValueOf(inv.ids).Len(); i++ {
			typeIds[typ][keyType][reflect.ValueOf(inv.ids).Index(i).Interface()] = true
		}
	}

	for typ, keyTypes := range typeInvocations {
		for keyType, invocations := range keyTypes {
			// get the ids for this type
			ids := reflect.New(reflect.SliceOf(keyType)).Elem()

			for id := range typeIds[typ][keyType] {
				ids = reflect.Append(ids, reflect.ValueOf(id))
			}

			resolved, err := l.register.resolve(ids.Interface(), typ)
			if err != nil {
				return err
			}

			// satisfy each invocation
			for _, invocation := range invocations {
				T := reflect.TypeOf(invocation.dst).Elem().Elem()
				pointer := reflect.TypeOf(invocation.dst).Elem().Elem().Kind() == reflect.Ptr
				if pointer {
					T = typ.Elem()
				}
				slice := reflect.New(reflect.SliceOf(T)).Elem()
				for i := 0; i < reflect.ValueOf(invocation.ids).Len(); i++ {
					v := resolved.MapIndex(reflect.ValueOf(invocation.ids).Index(i))
					if v.Interface() == nil {
						return errors.New("map index not found")
					}

					if !pointer {
						v = v.Elem()
					}

					slice = reflect.Append(slice, v)
				}

				reflect.ValueOf(invocation.dst).Elem().Set(slice)
			}
		}
	}

	return nil
}

// Resolves the ids by calling the resolver for the passed type T or a resolver
// for the pointer to passed type T. Returns a reflection of map[int64]*T.
func (l *register) resolve(ids interface{}, typ reflect.Type) (reflect.Value, error) {
	// TODO: Check if ids is slice?

	resolvers, ok := l.resolvers[typ]
	if !ok {
		if resolvers, ok = l.resolvers[reflect.PtrTo(typ)]; !ok {
			return reflect.Value{}, fmt.Errorf("no resolvers found for %v", typ.String())
		}
	}

	fn, ok := resolvers[reflect.TypeOf(ids).Elem()]
	if !ok {
		return reflect.Value{}, fmt.Errorf("no resolvers found for %v with key type %v", typ.String(), reflect.TypeOf(ids).Elem().String())
	}

	ldr := &loader{*l, nil}
	vals, err := fn(context.TODO(), ldr, ids)
	if err != nil {
		return reflect.Value{}, err
	}
	if err := ldr.execute(); err != nil {
		return reflect.Value{}, err
	}

	refVals := reflect.ValueOf(vals)
	if refVals.Len() != reflect.ValueOf(ids).Len() {
		return reflect.Value{}, errors.New("not all items found")
	}

	return refVals, nil
}

type invocation struct {
	ids interface{}
	dst interface{}
}
