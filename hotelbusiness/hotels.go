//go:build !solution

package hotelbusiness

import (
	"sort"
)

type Guest struct {
	CheckInDate  int
	CheckOutDate int
}

type Load struct {
	StartDate  int
	GuestCount int
}

type Event struct {
	IsComeIn bool
	Date     int
}

func ComputeLoad(guests []Guest) []Load {
	events := []Event{}
	for _, guest := range guests {
		events = append(events, Event{IsComeIn: true, Date: guest.CheckInDate})
		events = append(events, Event{IsComeIn: false, Date: guest.CheckOutDate})
	}
	sort.Slice(events, func(i, j int) bool {
		return events[i].Date < events[j].Date
	})
	loads := []Load{}
	if len(events) == 0 {
		return loads
	}

	cur_date := events[0].Date
	cur_cnt := 0
	prev_cnt := -1
	for _, event := range events {
		if event.Date > cur_date {
			if prev_cnt != cur_cnt {
				loads = append(loads, Load{StartDate: cur_date, GuestCount: cur_cnt})
				prev_cnt = cur_cnt
			}
			cur_date = event.Date
		}
		if event.IsComeIn {
			cur_cnt += 1
		} else {
			cur_cnt -= 1
		}
	}
	loads = append(loads, Load{StartDate: cur_date, GuestCount: cur_cnt})

	return loads
}
