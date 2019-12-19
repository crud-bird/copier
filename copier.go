package copier

import (
	"database/sql"
	"errors"
	"reflect"
	"strconv"
	"time"
)

//CopyOpt 自定义拷贝规则和tag
type CopyOpt struct {
	SetFunc func(toVal, fromVal reflect.Value)
	Tag     string
}

// Copy 扩展github.com/jinzhu/copier.Copy()函数，
// 不加标签时和原函数功能相同，
// 使用标签(默认使用copy,可自定义，opt.Tag)指定另一个结构体的字段名，使不同名称或者不同类型的字段相关联使字段相关联。
// 可以自定义拷贝规则(opt.SetFunc)，默认拷贝函数支持类型转换,包括：
// 1.字符串和数字类型相互转换
// 2.time.Time和字符串或者数字相互转换
// example:
// type To struct {
// 	A int `copy:"B"`
// 	T time.Time
// }
// type From struct {
// 	B  string
// 	Tt string `copy:"T"`
// }
// 调用Copy函数时，字段A,B相关联，T能转成字符串赋值给Tt
func Copy(toValue, fromValue interface{}, opt ...CopyOpt) (err error) {
	var (
		isSlice bool
		amount  = 1
		from    = indirect(reflect.ValueOf(fromValue))
		to      = indirect(reflect.ValueOf(toValue))
		tag     = "copy"
		setFunc = setFunc
	)

	if len(opt) > 0 {
		if opt[0].Tag != "" {
			tag = opt[0].Tag
		}
		if opt[0].SetFunc != nil {
			setFunc = opt[0].SetFunc
		}
	}

	if !to.CanAddr() {
		return errors.New("copy to value is unaddressable")
	}

	// Return is from value is invalid
	if !from.IsValid() {
		return
	}

	// Just set it if possible to assign
	if from.Type().AssignableTo(to.Type()) {
		to.Set(from)
		return
	}

	fromType := indirectType(from.Type())
	toType := indirectType(to.Type())

	if fromType.Kind() != reflect.Struct || toType.Kind() != reflect.Struct {
		return
	}

	if to.Kind() == reflect.Slice {
		isSlice = true
		if from.Kind() == reflect.Slice {
			amount = from.Len()
		}
	}

	var toTaggedFields, fromTaggedFields []reflect.StructField
	for i := 0; i < amount; i++ {
		var dest, source reflect.Value

		if isSlice {
			// source
			if from.Kind() == reflect.Slice {
				source = indirect(from.Index(i))
			} else {
				source = indirect(from)
			}

			// dest
			dest = indirect(reflect.New(toType).Elem())
		} else {
			source = indirect(from)
			dest = indirect(to)
		}

		// Copy from field to field or method
		for _, field := range deepFields(fromType) {
			//tagged field, deal with user-defined function
			if field.Tag.Get(tag) != "" {
				fromTaggedFields = append(fromTaggedFields, field)
				continue
			}

			name := field.Name

			if fromField := source.FieldByName(name); fromField.IsValid() {
				// has field
				if toField := dest.FieldByName(name); toField.IsValid() {
					if toField.CanSet() {
						if !set(toField, fromField) {
							if err := Copy(toField.Addr().Interface(), fromField.Interface()); err != nil {
								return err
							}
						}
					}
				} else {
					// try to set to method
					var toMethod reflect.Value
					if dest.CanAddr() {
						toMethod = dest.Addr().MethodByName(name)
					} else {
						toMethod = dest.MethodByName(name)
					}

					if toMethod.IsValid() && toMethod.Type().NumIn() == 1 && fromField.Type().AssignableTo(toMethod.Type().In(0)) {
						toMethod.Call([]reflect.Value{fromField})
					}
				}
			}
		}

		// Copy from method to field
		for _, field := range deepFields(toType) {
			//tagged field, deal with user-defined function
			if field.Tag.Get(tag) != "" {
				toTaggedFields = append(toTaggedFields, field)
				continue
			}

			name := field.Name

			var fromMethod reflect.Value
			if source.CanAddr() {
				fromMethod = source.Addr().MethodByName(name)
			} else {
				fromMethod = source.MethodByName(name)
			}

			if fromMethod.IsValid() && fromMethod.Type().NumIn() == 0 && fromMethod.Type().NumOut() == 1 {
				if toField := dest.FieldByName(name); toField.IsValid() && toField.CanSet() {
					values := fromMethod.Call([]reflect.Value{})
					if len(values) >= 1 {
						set(toField, values[0])
					}
				}
			}
		}

		if isSlice {
			if dest.Addr().Type().AssignableTo(to.Type().Elem()) {
				to.Set(reflect.Append(to, dest.Addr()))
			} else if dest.Type().AssignableTo(to.Type().Elem()) {
				to.Set(reflect.Append(to, dest))
			}
		}

		//user-defined rule
		for _, field := range fromTaggedFields {
			if fromField := source.FieldByName(field.Name); fromField.IsValid() {
				if toField := dest.FieldByName(field.Tag.Get(tag)); toField.IsValid() && toField.CanSet() {
					setFunc(toField, fromField)
				}
			}
		}
		for _, field := range toTaggedFields {
			if toField := dest.FieldByName(field.Name); toField.IsValid() && toField.CanSet() {
				if fromField := source.FieldByName(field.Tag.Get(tag)); fromField.IsValid() {
					setFunc(toField, fromField)
				}
			}
		}
	}
	return
}

func set(to, from reflect.Value) bool {
	if from.IsValid() {
		if to.Kind() == reflect.Ptr {
			//set `to` to nil if from is nil
			if from.Kind() == reflect.Ptr && from.IsNil() {
				to.Set(reflect.Zero(to.Type()))
				return true
			} else if to.IsNil() {
				to.Set(reflect.New(to.Type().Elem()))
			}
			to = to.Elem()
		}

		if from.Type().ConvertibleTo(to.Type()) {
			to.Set(from.Convert(to.Type()))
		} else if scanner, ok := to.Addr().Interface().(sql.Scanner); ok {
			err := scanner.Scan(from.Interface())
			if err != nil {
				return false
			}
		} else if from.Kind() == reflect.Ptr {
			return set(to, from.Elem())
		} else {
			return false
		}
	}
	return true
}

func deepFields(reflectType reflect.Type) []reflect.StructField {
	var fields []reflect.StructField

	if reflectType = indirectType(reflectType); reflectType.Kind() == reflect.Struct {
		for i := 0; i < reflectType.NumField(); i++ {
			v := reflectType.Field(i)
			if v.Anonymous {
				fields = append(fields, deepFields(v.Type)...)
			} else {
				fields = append(fields, v)
			}
		}
	}

	return fields
}

func indirect(reflectValue reflect.Value) reflect.Value {
	for reflectValue.Kind() == reflect.Ptr {
		reflectValue = reflectValue.Elem()
	}
	return reflectValue
}

func indirectType(reflectType reflect.Type) reflect.Type {
	for reflectType.Kind() == reflect.Ptr || reflectType.Kind() == reflect.Slice {
		reflectType = reflectType.Elem()
	}
	return reflectType
}

func setFunc(to, from reflect.Value) {
	if !(from.IsValid() && to.IsValid() && to.CanSet()) {
		return
	}

	typeTo := to.Type()
	typeFrom := from.Type()
	typeInt64 := reflect.TypeOf(int64(0))
	typeTime := reflect.TypeOf(time.Time{})

	//能直接复制
	if typeFrom.ConvertibleTo(typeTo) {
		to.Set(from.Convert(typeTo))
		return
	}

	//字符串和数字可以转换为时间
	if typeTo == typeTime {
		if typeFrom.ConvertibleTo(typeInt64) {
			t := time.Unix(from.Int(), 0)
			to.Set(reflect.ValueOf(t))
			return
		}

		if typeFrom.Kind() == reflect.String {
			if t, err := time.Parse("2006-01-02 15:04:05", from.String()); err == nil {
				to.Set(reflect.ValueOf(t))
				return
			}
		}
	}

	//时间可以转换为字符串或数字
	if typeFrom == typeTime {
		if t, ok := from.Interface().(time.Time); ok {
			if typeTo.ConvertibleTo(typeInt64) {
				to.Set(reflect.ValueOf(t.Unix()).Convert(typeTo))
				return
			}

			if typeTo.Kind() == reflect.String {
				to.Set(reflect.ValueOf(t.Format("2006-01-02 15:04:05")))
				return
			}
		}
	}

	//数字类型转字符串
	if typeTo.Kind() == reflect.String && typeFrom.ConvertibleTo(typeInt64) {
		to.Set(reflect.ValueOf(strconv.FormatInt(from.Int(), 10)))
		return
	}

	//字符串转数字
	if typeTo.ConvertibleTo(typeInt64) && typeFrom.Kind() == reflect.String {
		val, _ := strconv.ParseInt(from.String(), 10, 64)
		to.Set(reflect.ValueOf(val).Convert(typeTo))
		return
	}
}
