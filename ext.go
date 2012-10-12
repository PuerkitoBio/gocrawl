package gocrawl

import (
	"fmt"
)

type Extension struct {
}

func (this *Extension) Visited() {
	fmt.Println("Extension.Visited")
}

func (this *Extension) Enqueued() {
	fmt.Println("Extension.Enqueued")
}

type MyExt struct {
	Extension
}

func (this *MyExt) Enqueued() {
	fmt.Println("MyExt.Enqueued")
}
