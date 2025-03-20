package main

import "slices"

type DomainSet map[string]bool

func (d DomainSet) Add(domain string) {
	d[domain] = true
}

func (d DomainSet) Remove(domain string) {
	delete(d, domain)
}

func (d DomainSet) Contains(domain string) bool {
	_, ok := d[domain]
	return ok
}

func (d DomainSet) ToSortedList() []string {
	list := make([]string, 0, len(d))
	for domain := range d {
		list = append(list, domain)
	}
	slices.Sort(list)
	return list
}
