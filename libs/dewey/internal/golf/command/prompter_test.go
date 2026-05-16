package command

import "testing"

func TestStubPrompterConfirm(t *testing.T) {
	p := StubPrompter{}
	_, err := p.Confirm("proceed?")
	if err == nil {
		t.Error("StubPrompter.Confirm should return error")
	}
}

func TestStubPrompterSelect(t *testing.T) {
	p := StubPrompter{}
	_, err := p.Select("choose:", []string{"a", "b"})
	if err == nil {
		t.Error("StubPrompter.Select should return error")
	}
}

func TestStubPrompterInput(t *testing.T) {
	p := StubPrompter{}
	_, err := p.Input("name?")
	if err == nil {
		t.Error("StubPrompter.Input should return error")
	}
}
