package seed

import (
	"context"
	"fmt"
	"testing"
)

func TestSeed(t *testing.T) {
	t.Run("test seed data", func(t *testing.T) {
		err := Seed(func(etc context.Context) (err error) {
			fmt.Println("hello! ")
			return nil
		})
		if err != nil {
			t.Error(err)
		}
	})
}
