package smolder_test

import (
	"github.com/DusanKasan/smolder"
	"testing"
)

type (
	User struct {
		ID        int64
		Name      string
		Campaigns []Campaign
	}

	Campaign struct {
		ID      int64
		Name    string
		Flights []Flight
	}

	Flight struct {
		ID      int64
		Name    string
		Coupons []Coupon
	}

	Coupon struct {
		ID    int64
		Name  string
		Clips []Clip
	}

	Clip struct {
		ID   int64
		Name string
	}
)

var DB = struct {
	Users []struct {
		ID          int64
		Name        string
		CampaignIDs []int64
	}
	Campaigns []struct {
		ID          int64
		Name        string
		FlightIDs []int64
	}
	Flights []struct {
		ID          int64
		Name        string
		CouponsIDs []int64
	}
	Coupons []struct {
		ID          int64
		Name        string
		ClipIDs []int64
	}
	Clips []struct {
		ID          int64
		Name        string
	}
}{
	Users: []struct {
		ID          int64
		Name        string
		CampaignIDs []int64
	}{
		{
			ID:1,
			Name:"jozino",
			CampaignIDs: []int64{1, 2},
		},
		{
			ID:2,
			Name:"ferino",
			CampaignIDs: []int64{2},
		},
	},
	Campaigns: []struct {
		ID        int64
		Name      string
		FlightIDs []int64
	}{
		{
			ID: 1,
			Name: "kampan",
			FlightIDs: []int64{1, 2},
		},
		{
			ID: 2,
			Name: "kampania",
			FlightIDs: []int64{1},
		},
	},
	Flights: []struct {
		ID         int64
		Name       string
		CouponsIDs []int64
	}{
		{
			ID:1,
			Name:"flajt",
			CouponsIDs: []int64{1, 2},
		},
		{
			ID:2,
			Name:"flajt numero dos",
			CouponsIDs: []int64{2},
		},
	},
	Coupons: []struct {
		ID      int64
		Name    string
		ClipIDs []int64
	}{
		{
			ID:1,
			Name:"kupon",
			ClipIDs: []int64{1},
		},
		{
			ID:2,
			Name:"kjupon",
			ClipIDs: []int64{1, 2},
		},
	},
	Clips: []struct {
		ID   int64
		Name string
	}{
		{
			ID:1,
			Name:"klippity",
		},
		{
			ID:2,
			Name:"klappity",
		},
	},
}

func TestDataLoading(t *testing.T) {
	loader := smolder.New()
	if err := loader.Register(func(l smolder.Loader, ids []int64) (map[int64]*User, error) {
		t.Log("Loading Users for IDs", ids)

		m := map[int64]*User{}
		for _, id := range ids {
			for _, u := range DB.Users {
				if u.ID == id {
					usr := &User{
						ID: u.ID,
						Name: u.Name,
					}

					l.Load(u.CampaignIDs, &usr.Campaigns)
					m[id] = usr
				}
			}
		}

		return m, nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := loader.Register(func(l smolder.Loader, ids []int64) (map[int64]*Campaign, error) {
		t.Log("Loading Campaigns for IDs", ids)

		m := map[int64]*Campaign{}
		for _, id := range ids {
			for _, u := range DB.Campaigns {
				if u.ID == id {
					usr := &Campaign{
						ID: u.ID,
						Name: u.Name,
					}

					l.Load(u.FlightIDs, &usr.Flights)
					m[id] = usr
				}
			}
		}

		return m, nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := loader.Register(func(l smolder.Loader, ids []int64) (map[int64]*Flight, error) {
		t.Log("Loading Flights for IDs", ids)

		m := map[int64]*Flight{}
		for _, id := range ids {
			for _, u := range DB.Flights {
				if u.ID == id {
					usr := &Flight{
						ID: u.ID,
						Name: u.Name,
					}

					l.Load(u.CouponsIDs, &usr.Coupons)
					m[id] = usr
				}
			}
		}

		return m, nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := loader.Register(func(l smolder.Loader, ids []int64) (map[int64]*Coupon, error) {
		t.Log("Loading Coupons for IDs", ids)

		m := map[int64]*Coupon{}
		for _, id := range ids {
			for _, u := range DB.Coupons {
				if u.ID == id {
					usr := &Coupon{
						ID: u.ID,
						Name: u.Name,
					}

					l.Load(u.ClipIDs, &usr.Clips)
					m[id] = usr
				}
			}
		}

		return m, nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := loader.Register(func(l smolder.Loader, ids []int64) (map[int64]*Clip, error) {
		t.Log("Loading Clips for IDs", ids)

		m := map[int64]*Clip{}
		for _, id := range ids {
			for _, u := range DB.Clips {
				if u.ID == id {
					usr := &Clip{
						ID: u.ID,
						Name: u.Name,
					}

					m[id] = usr
				}
			}
		}

		return m, nil
	}); err != nil {
		t.Fatal(err)
	}

	usrs := []User{}
	err := loader.Load([]int64{1, 2}, &usrs)
	if err != nil {
		t.Fatal(err)
	}

	for _, u := range usrs {
		t.Logf("%#v\n",u)
	}
}

