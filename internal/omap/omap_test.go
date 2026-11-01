package omap

import (
	"reflect"
	"testing"
)

func TestKeepsInsertionOrder(t *testing.T) {
	m := Of[any]("b", 1, "a", nil)
	m.Set("c", 3)
	m.Set("b", 2)

	if !reflect.DeepEqual(m.Keys(), []string{"b", "a", "c"}) || m.Value("b") != 2 || !m.Has("a") || m.Value("a") != nil {
		t.Errorf("keys %v", m.Keys())
	}

	m.Delete("a")
	clone := m.Copy()
	clone.Set("d", 4)

	if !reflect.DeepEqual(m.Keys(), []string{"b", "c"}) || m.Len() != 2 || clone.Len() != 3 {
		t.Errorf("after delete %v, clone %v", m.Keys(), clone.Keys())
	}
}
