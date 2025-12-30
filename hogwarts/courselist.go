//go:build !solution

package hogwarts

// package main

import (
	"sort"
)

type Timings struct {
	Name    string
	TimeIn  int
	TimeOut int
}

func Visit(name string, cur_time *int, prereqs map[string][]string, times map[string]Timings) {
	// fmt.Printf("Visit course %s | Cur time is %d\n", name, *cur_time)

	this_time := times[name]
	this_time.TimeIn = *cur_time
	this_time.Name = name
	times[name] = this_time
	*cur_time++

	deps := prereqs[name]
	dep_time := Timings{}
	for _, dep_name := range deps {
		dep_time = times[dep_name]
		if dep_time.TimeOut != 0 {
			continue
		}
		if dep_time.TimeIn != 0 {
			panic("Cycle Deps!!!")
		}
		Visit(dep_name, cur_time, prereqs, times)
	}

	this_time.TimeOut = *cur_time
	times[name] = this_time
	*cur_time++
}

func GetCourseList(prereqs map[string][]string) []string {
	times := make(map[string]Timings)
	cur_time := 1
	for name, _ := range prereqs {
		if times[name].TimeOut != 0 {
			continue
		}
		Visit(name, &cur_time, prereqs, times)
	}
	// fmt.Println("Get times:")
	var course_list []Timings
	for _, timing := range times {
		course_list = append(course_list, timing)
		// fmt.Printf("  %s: TimeIn=%d, TimeOut=%d\n", timing.Name, timing.TimeIn, timing.TimeOut)
	}
	sort.Slice(course_list, func(i, j int) bool {
		return course_list[i].TimeOut < course_list[j].TimeOut
	})
	var res_course_list []string
	for _, timing := range course_list {
		res_course_list = append(res_course_list, timing.Name)
		// fmt.Printf("  %s: TimeIn=%d, TimeOut=%d\n", timing.Name, timing.TimeIn, timing.TimeOut)
	}

	return res_course_list
}

// func main() {
// 	var computerScience = map[string][]string{
// 		"algorithms": {"data structures"},
// 		"calculus":   {"linear algebra"},
// 		"compilers": {
// 			"data structures",
// 			"formal languages",
// 			"computer organization",
// 		},
// 		"data structures":       {"discrete math"},
// 		"databases":             {"data structures"},
// 		"discrete math":         {"intro to programming"},
// 		"formal languages":      {"discrete math"},
// 		"networks":              {"operating systems"},
// 		"operating systems":     {"data structures", "computer organization"},
// 		"programming languages": {"data structures", "computer organization"},
// 	}

// 	res := GetCourseList(computerScience)

// 	fmt.Printf("res: %s", res)
// }
