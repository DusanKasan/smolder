package smolder

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

type register struct {
	resolvers map[reflect.Type]func(context.Context, *loader, []int64) (interface{}, error)
}

func New() *register {
	m := map[reflect.Type]func(context.Context, *loader, []int64) (interface{}, error){}
	return &register{resolvers: m}
}

// fn for type T must be:
// 		func([]int64) (map[int64]*T, error)
// 		func(context.Context, []int64) (map[int64]*T, error)
// 		func(smolder.loader, []int64) (map[int64]*T, error)
// 		func(context.Context, smolder.loader, []int64) (map[int64]*T, error)
// 		func([]int64) map[int64]*T
// 		func(context.Context, []int64) map[int64]*T
// 		func(loader, []int64) map[int64]*T
// 		func(context.Context, smolder.loader, []int64) map[int64]*T
func (l *register) Register(fn interface{}) error {
	var inTransform func(ctx context.Context, loader *loader, ids []int64) []reflect.Value
	var outTransform func(vals []reflect.Value) (interface{}, error)

	t := reflect.TypeOf(fn)
	if t.Kind() != reflect.Func {
		return errors.New("fn must be a function")
	}

	switch t.NumOut() {
	case 1:
		if t.Out(0).Kind() != reflect.Map || t.Out(0).Key().Kind() != reflect.Int64 {
			return errors.New("fn's first output must be a map with keys of int64")
		}
		outTransform = func(vals []reflect.Value) (interface{}, error) {
			return vals[0].Interface(), nil
		}
	case 2:
		if t.Out(0).Kind() != reflect.Map || t.Out(0).Key().Kind() != reflect.Int64 {
			return errors.New("fn's first output must be a map with keys of int64")
		}

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
		if t.In(0).Kind() != reflect.Slice || t.In(0).Elem().Kind() != reflect.Int64 {
			return errors.New("fn's first argument must be []int64")
		}
		inTransform = func(ctx context.Context, loader *loader, ids []int64) []reflect.Value {
			return []reflect.Value{
				reflect.ValueOf(ids),
			}
		}
	case 2:
		if t.In(1).Kind() != reflect.Slice || t.In(1).Elem().Kind() != reflect.Int64 {
			return errors.New("fn's second argument must be []int64")
		}

		if t.In(0).AssignableTo(reflect.TypeOf((*context.Context)(nil)).Elem()) {
			inTransform = func(ctx context.Context, loader *loader, ids []int64) []reflect.Value {
				return []reflect.Value{
					reflect.ValueOf(ctx),
					reflect.ValueOf(ids),
				}
			}
		} else if t.In(0).AssignableTo(reflect.TypeOf((*Loader)(nil)).Elem()) {
			inTransform = func(ctx context.Context, loader *loader, ids []int64) []reflect.Value {
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

		if t.In(2).Kind() != reflect.Slice || t.In(2).Elem().Kind() != reflect.Int64 {
			return errors.New("fn's third argument must be []int64")
		}
		inTransform = func(ctx context.Context, loader *loader, ids []int64) []reflect.Value {
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
	l.resolvers[typ] = func(ctx context.Context, loader *loader, ids []int64) (interface{}, error) {
		return outTransform(reflect.ValueOf(fn).Call(inTransform(ctx, loader, ids)))
	}

	return nil
}

func (l *register) Load(ids []int64, dst interface{}) error {
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
		Load(ids []int64, dst interface{})
	}

	loader struct {
		register    register
		invocations []invocation
	}
)

func (l *loader) Load(ids []int64, dst interface{}) {
	l.invocations = append(l.invocations, invocation{ids, dst})
}

func (l *loader) execute() error {
	// Group the invocations by the type they want to resolve and group the IDs
	// into distinct groups by the type.
	typeInvocations := map[reflect.Type][]invocation{}
	typeIds := map[reflect.Type]map[int64]bool{}
	for _, inv := range l.invocations {
		typ := reflect.TypeOf(inv.dst).Elem().Elem()
		if typ.Kind() != reflect.Ptr {
			typ = reflect.PtrTo(typ)
		}

		typeInvocations[typ] = append(typeInvocations[typ], inv)
		if _, ok := typeIds[typ]; !ok {
			typeIds[typ] = map[int64]bool{}
		}

		for _, id := range inv.ids {
			typeIds[typ][id] = true
		}
	}

	for typ, invocations := range typeInvocations {
		// get the ids for this type
		var ids []int64
		for id := range typeIds[typ] {
			ids = append(ids, id)
		}

		resolved, err := l.register.resolve(ids, typ)
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
			for _, id := range invocation.ids {
				v := resolved.MapIndex(reflect.ValueOf(id))
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

	return nil
}

// Resolves the ids by calling the resolver for the passed type T or a resolver
// for the pointer to passed type T. Returns a reflection of map[int64]*T.
func (l *register) resolve(ids []int64, typ reflect.Type) (reflect.Value, error) {
	fn, ok := l.resolvers[typ]
	if !ok {
		if fn, ok = l.resolvers[reflect.PtrTo(typ)]; !ok {
			return reflect.Value{}, fmt.Errorf("no resolver found for %v", typ.String())

		}
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
	if refVals.Len() != len(ids) {
		return reflect.Value{}, errors.New("not all items found")
	}

	return refVals, nil
}

type invocation struct {
	ids []int64
	dst interface{}
}
