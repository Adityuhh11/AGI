package notes

import "testing"

func TestAServiceWithoutAFolderSaysSoInsteadOfCrashing(t *testing.T) {
	s := NewService(nil, nil)
	if _, err := s.List(""); err == nil {
		t.Error("List")
	}
	if _, err := s.Get("20261026-231500"); err == nil {
		t.Error("Get")
	}
	if _, err := s.Save("", "x"); err == nil {
		t.Error("Save")
	}
	if err := s.Delete("20261026-231500"); err == nil {
		t.Error("Delete")
	}
	s.SetOnTop(true) // nothing to move, nothing to fail
}

func TestAServiceCarriesTheStoreAndTheOnTopSwitch(t *testing.T) {
	var onTop []bool
	s := NewService(newStore(t), func(on bool) { onTop = append(onTop, on) })
	n, err := s.Save("", "hello")
	if err != nil || n.ID == "" {
		t.Fatalf("Save = %+v, %v", n, err)
	}
	if list, err := s.List("hell"); err != nil || len(list) != 1 {
		t.Fatalf("List = %v, %v", list, err)
	}
	s.SetOnTop(false)
	if len(onTop) != 1 || onTop[0] {
		t.Errorf("on top calls %v", onTop)
	}
}
