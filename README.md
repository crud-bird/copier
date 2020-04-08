# Copy
+ 扩展[copier.Copy](https://github.com/jinzhu/copier)函数，不加标签时和原函数功能相同。
+ 使用标签(默认使用copy,可自定义，opt.Tag)指定另一个结构体的字段名，使不同名称或者不同类型的字段相关联使字段相关联。
+ 可以自定义拷贝规则(opt.CopyRule)，默认拷贝规则支持类型转换,包括：
    + 字符串和数字类型相互转换
    + time.Time和字符串或者数字相互转换

## Example: 
```
import (
    "time"
    "reflect"
    "github.com/crud-bird/copier"
)

var RoleMap = map[int64]string{
    2200: "mid",
    2800: "ad",
    4396: "jg",
}

//tag可以写在任意一个结构体的定义上
type UserA struct {
    //关联UserB的Id字段
    ID   int64 `copy:"Id"`
    Name string
    Role string
    //两个结构体的CreatedAt字段类型不一样，加tag可以在拷贝中转换类型
    CreatedAt string `copy:"CreatedAt"`
}

type UserB struct {
    Id     int32
    Name   string
    //关联UserA的Role字段
    RoleId int64 `copy:"Role"`
    CreatedAt time.Time
}

func TestCopy() {
    //简单结构体
    uA := UserA{}
    uB := UserB{7, "clearlove", 4396, time.Now()}
    common.Copy(&uA, &uB, common.CopyOpt{CopyRule: copyRule})

    //数组
    uAs := make([]UserA, 0)
    uBs := []UserB{{1, "uzi", 2800, time.Now()}, {2, "xiaohu", 2200, time.Now()}}
    common.Copy(&uAs, &uBs, common.CopyOpt{CopyRule: copyRule})

    //指针数组也同样支持，toValue和fromValue无论是指针数组还是对象数组都能正常拷贝
    as := make([]*UserA, 0)
    bs := []UserB{{1, "uzi", 2800, time.Now()}, {2, "xiaohu", 2200, time.Now()}}
    common.Copy(&as, &bs, common.CopyOpt{CopyRule: copyRule})
}

//自定义拷贝规则
func copyRule(to, from reflect.Value, field string) bool {
    //特殊处理RoleId字段，为tag定义在UserB上，所以这里的field是RoleId而不是Role
    if field == "RoleId" {
    	to.Set(reflect.ValueOf(RoleMap[from.Int()]))
    	return true
    }
    return false
}
```
