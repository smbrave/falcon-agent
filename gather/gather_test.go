package gather

import (
	"testing"
)

func TestGather(t *testing.T) {
	cfg := `{"enable": 1, "files":[
		{"file":"./test.log","items":[{"metric":"a.b.c"},{"metric":"a.b.d"}]},
		{}
	]}`

	Init(cfg)
	Run()

}
