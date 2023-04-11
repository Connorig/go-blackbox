package appioc

import (
	"testing"
)

type Stu struct {
	Name string
	Age  int
}

func TestName(t *testing.T) {
	stu1 := Stu{
		Name: "Homelander",
		Age:  18,
	}
	err := Set2("stu", &stu1)
	if err != nil {
		t.Error(err)
	}

	bean := Get2("stu")
	if bean != nil {
		//t.Log(stu1 == bean)
		//t.Logf("%p", &stu1)
		//t.Logf("%p", bean)

	}
}

func TestName2(t *testing.T) {
	stu1 := Stu{
		Name: "Homelander",
		Age:  18,
	}
	Set(&stu1)

	get := Get((*Stu)(nil))
	t.Logf("%p", get)
	t.Logf("%p", &stu1)
}
