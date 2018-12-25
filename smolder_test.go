package smolder_test

import (
	"encoding/json"
	"fmt"
	"github.com/DusanKasan/smolder"
	"testing"
)

type (
	Uuser struct {
		ID int64
		Name string
		Addresses []Address
		Roles []Role
	}
	Address struct {
		ID int64
		Street string
		Country Country
	}
	Country struct {
		ID int64
		name string
	}
	Role struct {
		Name string
	}
)

var db = struct{
	Users []struct{
		ID int64
		Name string
		AddressIDs []int64
	}
	Addresses []struct{
		ID int64
		Street string
		CountryID int64
	}
	Countries []struct{
		ID int64
		Name string
	}
	UserRoles []struct{
		UserID int64
		Role string
	}
}{
	Users: []struct {
		ID         int64
		Name       string
		AddressIDs []int64
	}{
		{1, "janko", []int64{1,2}},
		{2, "ferko", []int64{2}},
		{3, "bobrze", []int64{3}},
		{4, "hryze123", []int64{3,4}},
		{5, "orban", []int64{4,5}},
		{6, "gabo", []int64{6}},
	},
	Addresses: []struct {
		ID        int64
		Street    string
		CountryID int64
	}{
		{1, "Hlavna", 1},
		{2, "Sturova", 1},
		{3, "Kurwa", 2},
		{4, "Chuju", 2},
		{5, "Petofi", 3},
		{6, "Lajosz", 3},
	},
	Countries: []struct {
		ID   int64
		Name string
	}{
		{1, "Slovakia"},
		{2, "Poland"},
		{3, "Hungary"},
	},
	UserRoles: []struct {
		UserID int64
		Role   string
	}{
		{1, "admin"},
		{1, "consumer"},
		{2, "consumer"},
		{3, "advertiser"},
		{4, "consumer"},
		{5, "consumer"},
		{5, "advertiser"},
		{6, "consumer"},
		{6, "tester"},
	},
}

func loadUsers(loader smolder.Loader, IDs []int64) map[int64]*Uuser {
	fmt.Println("load users ", IDs)
	users := map[int64]*Uuser{}

	for _, u := range db.Users {
		for _, id := range IDs {
			if u.ID == id {
				usr := &Uuser{
					ID: u.ID,
					Name: u.Name,
				}

				loader.Load(u.AddressIDs, &usr.Addresses)
				loader.Load(UserId{usr.ID}, &usr.Roles)
				users[id] = usr
			}
		}
	}

	return users
}

func loadAddress(loader smolder.Loader, IDs []int64) map[int64]*Address {
	fmt.Println("load address ", IDs)

	addresses := map[int64]*Address{}

	for _, a := range db.Addresses {
		for _, id := range IDs {
			if a.ID == id {
				addr := &Address{
					ID: a.ID,
					Street: a.Street,
				}

				loader.Load(a.CountryID, &addr.Country)
				addresses[id] = addr
			}
		}
	}

	return addresses
}

func loadCountries(IDs []int64) map[int64]*Country {
	fmt.Println("load countries ", IDs)
	countries := map[int64]*Country{}

	for _, c := range db.Countries {
		for _, id := range IDs {
			if c.ID == id {
				countries[id] = &Country{
					ID: c.ID,
					name: c.Name,
				}
			}
		}
	}

	return countries
}

type UserId struct {
	ID int64
}

func loadRoles(users []UserId) map[UserId][]*Role {
	fmt.Println("load roles ", users)
	roles := map[UserId][]*Role{}

	for _, r := range db.UserRoles {
		for _, u := range users {
			if u.ID == r.UserID {
				roles[u] = append(roles[u], &Role{
					Name: r.Role,
				})
			}
		}
	}

	return roles
}

func TestComprehensive(t *testing.T) {
	loader := smolder.New()
	if err := loader.Register(loadUsers); err != nil {
		t.Fatal(err)
	}
	if err := loader.Register(loadAddress); err != nil {
		t.Fatal(err)
	}
	if err := loader.Register(loadCountries); err != nil {
		t.Fatal(err)
	}
	if err := loader.Register(loadRoles); err != nil {
		t.Fatal(err)
	}

	var u []Uuser
	err := loader.Load([]int64{1, 2}, &u)
	if err != nil {
		t.Fatal(err)
	}

	b, err := json.MarshalIndent(u, "", "\t")
	if err != nil {
		t.Fatal(err)
	}

	t.Log(string(b))
}