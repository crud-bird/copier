package copier_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/crud-bird/copier"
)

var roleMap = map[int64]string{
	2200: "mid",
	2800: "ad",
	4396: "jg",
}

type userA struct {
	ID        int64 `copy:"Id"`
	Name      string
	Role      string
	CreatedAt string `copy:"CreatedAt"`
}

type userB struct {
	Id        int32
	Name      string
	RoleId    int64 `copy:"Role"`
	CreatedAt time.Time
}

func TestCopy(t *testing.T) {
	uA := userA{}
	uB := userB{7, "clearlove", 4396, time.Now()}
	copier.Copy(&uA, &uB, copier.CopyOpt{CopyRule: copyRule})
	t.Logf("%+v", uA)

	//指针数组和对象数组都能拷贝
	uAs := make([]*userA, 0)
	uBs := []userB{{1, "uzi", 2800, time.Now()}, {2, "xiaohu", 2200, time.Now()}}
	copier.Copy(&uAs, &uBs, copier.CopyOpt{CopyRule: copyRule})
	for _, u := range uAs {
		t.Logf("%+v", u)
	}
}

func copyRule(to, from reflect.Value, field string) bool {
	if field == "RoleId" {
		to.Set(reflect.ValueOf(roleMap[from.Int()]))
		return true
	}
	return false
}
