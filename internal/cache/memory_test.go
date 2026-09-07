package cache

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestMemoryBoundsCopiesExpiryAndConcurrentAccess(t *testing.T) {
	m := New()
	now := time.Now()
	m.Now = func() time.Time { return now }
	original := []byte("data")
	m.Put("work:item", original, time.Second)
	original[0] = 'X'
	b, _, _, ok := m.Get("work:item", false)
	if !ok || string(b) != "data" {
		t.Fatal(string(b))
	}
	b[0] = 'Y'
	b, _, _, _ = m.Get("work:item", false)
	if string(b) != "data" {
		t.Fatal("read alias")
	}
	now = now.Add(time.Second)
	if _, _, _, ok = m.Get("work:item", false); ok {
		t.Fatal("expired hit")
	}
	if _, _, stale, ok := m.Get("work:item", true); !ok || !stale {
		t.Fatal("offline expired miss")
	}
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			key := fmt.Sprint(i)
			m.Put(key, []byte("value"), time.Minute)
			m.Get(key, false)
		}()
	}
	wg.Wait()
	if len(m.entries) > 128 || m.bytes < 0 {
		t.Fatal("unbounded cache")
	}
	m.Put("same", []byte("long"), time.Minute)
	m.Put("same", []byte("s"), time.Minute)
	actual := 0
	for _, e := range m.entries {
		actual += len(e.Value)
	}
	if actual != m.bytes {
		t.Fatal("wrong accounting", actual, m.bytes)
	}
	m.DeletePrefix("same")
	if _, _, _, ok := m.Get("same", false); ok {
		t.Fatal("prefix retained")
	}
}
