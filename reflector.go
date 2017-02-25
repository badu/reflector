package reflector

import (
	"fmt"
	"reflect"
	"time"
)

func (m *Reflector) ComponentsScan(components ...interface{}) error {
	for _, component := range components {
		value := reflect.ValueOf(component)
		valueType := value.Type()

		if value.Kind() != reflect.Struct {
			return fmt.Errorf("We don't visit `%v` kind here!", value.Kind())
		}

		// set current model
		m.currentModel = &Model{ModelType: valueType, printNesting: 0}

		err := m.visit(value)

		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Reflector) inspectMap(value reflect.Value) error {

	for _, key := range value.MapKeys() {
		keyValue := value.MapIndex(key)

		if err := m.inspectMapKeyValue(value, key, keyValue); err != nil {
			return err
		}

		if err := m.inspect(key); err != nil {
			return err
		}

		if err := m.inspect(keyValue); err != nil {
			return err
		}

	}

	return nil
}

func (m *Reflector) inspectSlice(value reflect.Value) error {

	for i := 0; i < value.Len(); i++ {
		elem := value.Index(i)

		if err := m.inspectSliceElem(i, elem); err != nil {
			return err
		}

		if err := m.inspect(elem); err != nil {
			return err
		}

	}

	return nil
}

func (m *Reflector) inspectArray(value reflect.Value) error {

	for i := 0; i < value.Len(); i++ {
		elem := value.Index(i)

		if err := m.InspectArrayElem(i, elem); err != nil {
			return err
		}

		if err := m.inspect(elem); err != nil {
			return err
		}

	}

	return nil
}

func (m *Reflector) inspectMapKeyValue(theMap, key, value reflect.Value) error {
	//fmt.Printf("MapElem %v %v\n", key, value)
	return nil
}

func (m *Reflector) inspectSliceElem(index int, value reflect.Value) error {
	fmt.Printf("SliceElem : #%d %v\n", index, value)
	return nil
}

func (m *Reflector) InspectArrayElem(index int, value reflect.Value) error {
	//fmt.Printf("ArrayElem: #%d %v\n", index, value)
	return nil
}

func (m *Reflector) inspect(value reflect.Value) error {
	// Determine if we're receiving a pointer and if so notify the
	// The logic here is convoluted but very important (tests will fail if
	// almost any part is changed).
	//
	// First, we check if the value is an interface, if so, we really need
	// to check the interface's value to see whether it is a pointer.
	//
	// Check whether the value is then a pointer. If so, then set pointer
	// to true to notify the
	//
	// If we still have a pointer or an interface after the indirections, then
	// we unwrap another level
	//
	// At this time, we also set "value" to be the de-referenced value. This is
	// because once we've unwrapped the pointer we want to use that value.
	var err error
	valuePtr := value
	isPointer := false

	for {
		switch valuePtr.Kind() {
		case reflect.Interface:
			valuePtr = valuePtr.Elem()
			// fallthrough, since it can be an interface
			fallthrough
		case reflect.Ptr:
			value = reflect.Indirect(valuePtr)
			valuePtr = value
			isPointer = true
		}

		// If we still have a pointer or interface we have to indirect another level.
		switch valuePtr.Kind() {
		case reflect.Ptr, reflect.Interface:
			continue
		}
		break
	}

	if value.Kind() == reflect.Interface {
		value = value.Elem()
	}

	kind := value.Kind()
	if kind >= reflect.Int && kind <= reflect.Complex128 {
		kind = reflect.Int
	}

	if printDebug {
		fmt.Printf("%s[level #%d] Current model type : %q\n", m.currentModel.tabs(), m.currentModel.printNesting, m.currentModel.ModelType)
	}

	switch kind {
	case reflect.Bool, reflect.Chan, reflect.Func, reflect.Int, reflect.String, reflect.Invalid:
		// Primitives
		return nil
	case reflect.Map:
		err = m.inspectMap(value)
		return err
	case reflect.Slice:
		err = m.inspectSlice(value)
		return err
	case reflect.Array:
		err = m.inspectArray(value)
		return err
	case reflect.Struct:
		if !isPointer {
			/**
			if printDebug{
				fmt.Printf("%sNot a ponter. inspecting struct %s\n",tabs, m.currentModel.Name)
			}
			**/
			err = m.inspectStruct(value)
			return err
		}
	default:
		return fmt.Errorf("Inspector : unsupported type %q ", kind.String())
	}
	return err
}

func (m *Reflector) inspectField(field reflect.StructField, value reflect.Value) (*Field, error) {
	var err error
	var pointedElement reflect.Type
	var pointedStruct reflect.Value

	// set current field as a new field
	result := &Field{
		Type:  field.Type,
		Name:  field.Name,
		Value: value,
	}

	result.Tags, err = result.ParseStructTag(string(field.Tag))
	if err != nil {
		return nil, err
	}

	if field.Anonymous {
		result.flags = result.flags | (1 << ff_is_anonymous)
	}

	switch field.Type.Kind() {
	case reflect.Ptr:
		// set flag to pointer
		result.flags = result.flags | (1 << ff_is_pointer)
		if printDebug {
			fmt.Printf("%s[PTR] %q (valid = %t , nil = %t) `%v`\n", m.currentModel.tabs(), field.Name, value.IsValid(), value.IsNil(), value)
		}
		// fallthrough, since it can be a pointer to a struct, slice, whatever
		fallthrough

	case reflect.Struct:
		// set flag to struct
		result.flags = result.flags | (1 << ff_is_struct)

		// if it's not a pointer
		if result.flags&(1<<ff_is_pointer) == 0 {
			/**
			if printDebug {
				fmt.Printf("%sSTRUCTFIELD %q : %q = `%v`\n", tabs, result.Name, m.currentModel.ModelType, value)
			}
			**/
			// TODO : add Scanner, Valuer, Marshaller, Unmarshaller
			_, isTime := value.Interface().(time.Time)
			if isTime {
				// set flag to time
				result.flags = result.flags | (1 << ff_is_time)
			}

			// check if it's self reference
			if m.currentModel.ModelType.Name() == result.Type.Name() {
				// set self referenced flag, so printing won't go in infinite loop
				result.flags = result.flags | (1 << ff_is_self_reference)
			}
			// set the relationship flag
			result.flags = result.flags | (1 << ff_is_relation)

			// Get cached Model
			if cachedValue := cachedModels.get(field.Type); cachedValue != nil {
				result.Relation = cachedValue
			} else {
				// set the relationship
				result.Relation = &Model{
					ModelType:    result.Type,
					Value:        value,
					printNesting: m.currentModel.printNesting + 1}
			}

		} else {
			// TODO : pointer to time.Time seems to escape

			// field is nil, we build one to visit it
			// it's a pointer. dereferencing it
			pointedElement = result.Type.Elem()
			pointedStruct = reflect.New(pointedElement).Elem()
			valueType := pointedStruct.Type()

			if printDebug {
				fmt.Printf("%sPTRSTRUCTFIELD %q %v\n", m.currentModel.tabs(), result.Name, valueType)
			}

			// check if it's self reference
			if m.currentModel.ModelType.Name() == valueType.Name() {
				// set self referenced flag, so printing won't go in infinite loop
				result.flags = result.flags | (1 << ff_is_self_reference)
			}

			// set the relationship flag
			result.flags = result.flags | (1 << ff_is_relation)

			// Get cached Model
			if cachedValue := cachedModels.get(valueType); cachedValue != nil {
				if printDebug {
					fmt.Printf("%sCACHED : %v\n", m.currentModel.tabs(), valueType)
				}
				result.Relation = cachedValue
			} else {

				// set the relationship
				result.Relation = &Model{
					ModelType:    valueType,
					Value:        pointedStruct,
					printNesting: m.currentModel.printNesting + 1}
			}

		}

	case reflect.Slice:

		if result.Type.Elem().Kind() == reflect.Ptr {
			pointedElement = result.Type.Elem().Elem()
			// set flag to pointer
			result.flags = result.flags | (1 << ff_is_pointer)
		} else {
			pointedElement = result.Type.Elem()
		}
		pointedStruct = reflect.New(pointedElement).Elem()

		// check for self reference
		if m.currentModel.ModelType.Name() == pointedStruct.Type().Name() {
			// set self referenced flag, so printing won't go in infinite loop
			result.flags = result.flags | (1 << ff_is_self_reference)
		}

		// set the flag
		result.flags = result.flags | (1 << ff_is_relation)
		// set flag to slice
		result.flags = result.flags | (1 << ff_is_slice)

		// Get cached Model
		if cachedValue := cachedModels.get(pointedElement); cachedValue != nil {
			result.Relation = cachedValue
		} else {
			result.Relation = &Model{
				ModelType:    pointedStruct.Type(),
				Value:        pointedStruct,
				printNesting: m.currentModel.printNesting + 1}

		}

		// inspect it
		err = m.inspect(value)

	case
		reflect.Invalid,
		reflect.Bool,
		reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64,
		reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64,
		reflect.Uintptr,
		reflect.Float32,
		reflect.Float64,
		reflect.Complex64,
		reflect.Complex128,
		reflect.String:
		// primitive
	default:
		if printDebug {
			fmt.Printf("%sDEFAULT %v of %s\n", m.currentModel.tabs(), value, result.Name)
		}
		// by default, we inspect
		err = m.inspect(value)

	}
	return result, err
}

func (m *Reflector) inspectStruct(value reflect.Value) error {

	valueType := value.Type()
	/**
	if printDebug {
		fmt.Printf("%sInspect struct %s\n", tabs, m.currentModel.Name)
	}
	**/
	var inspectedField *Field
	var err error

	for i := 0; i < valueType.NumField(); i++ {
		structField := valueType.Field(i)
		field := value.FieldByIndex([]int{i})
		fieldType := field.Type()

		if structField.Anonymous {
			for j := 0; j < field.NumField(); j++ {
				subStructField := fieldType.Field(j)
				subField := field.FieldByIndex([]int{j})
				if subStructField.Anonymous {
					return fmt.Errorf("We don't support (for now) two levels of anonymity")
				}
				/**
				if printDebug {
					fmt.Printf("%s[ANON] %q %q = %v\n", tabs, subStructField.Name, subStructField.Type, subField)
				}
				**/
				// force anonymous to true, because it has to set the flag
				subStructField.Anonymous = true
				inspectedField, err = m.inspectField(subStructField, subField)

				if err == nil {
					// add the field to current model
					m.currentModel.addField(inspectedField)
				}
			}
		} else {
			/**
			if printDebug {
				fmt.Printf("%s[Field] Current field %q : %q\n", tabs, structField.Name, structField.Type)
			}
			**/
			if field.Kind() == reflect.Invalid {
				if printDebug {
					fmt.Printf("%sINVALID FIELD : %q = %q\n", m.currentModel.tabs(), structField.Name, structField.Type)
				}
				continue
			}

			inspectedField, err = m.inspectField(structField, field)

			if err == nil {
				// add the field to current model
				m.currentModel.addField(inspectedField)
			}
		}

	}

	return nil
}

func (m *Reflector) visit(value reflect.Value) error {
	var err error

	valueType := value.Type()

	// Get cached Model
	if cachedValue := cachedModels.get(valueType); cachedValue != nil {
		return nil
	}

	switch value.Kind() {
	case
		reflect.Invalid,
		reflect.Bool,
		reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64,
		reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64,
		reflect.Uintptr,
		reflect.Float32,
		reflect.Float64,
		reflect.Complex64,
		reflect.Complex128,
		reflect.String:
		// do nothing, it's primitive
	default:
		if printDebug {
			fmt.Printf("%s->Visiting %s\n", m.currentModel.tabs(), m.currentModel.ModelType)
		}
		// inspect the value
		err = m.inspect(value)

		m.currentModel.visited = true
		// Set cached model
		cachedModels.set(valueType, m.currentModel)

		if printDebug {
			fmt.Printf("%s<-Finished Visiting %s\n\n", m.currentModel.tabs(), m.currentModel.ModelType)
		}

	}

	for _, field := range m.currentModel.Fields {
		if field.HasRelation() {
			if !field.Relation.visited {

				if printDebug {
					fmt.Printf("%s%q %q -> %q\n", m.currentModel.tabs(), field.Name, field.Type, field.Relation.ModelType)
				}
				// TODO : check where the model name is set wrong (see Street struct)
				newReflector := &Reflector{}
				newReflector.currentModel = field.Relation
				err = newReflector.visit(field.Relation.Value)
				if err != nil {
					return err
				}
				field.Relation.visited = true
			}
		}
	}

	return err

}
