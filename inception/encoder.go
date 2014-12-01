/**
 *  Copyright 2014 Paul Querna
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package ffjsoninception

import (
	"fmt"
	"reflect"
)

func typeInInception(ic *Inception, typ reflect.Type) bool {
	for _, v := range ic.objs {
		if v.Typ == typ {
			return true
		}
		if typ.Kind() == reflect.Ptr {
			if v.Typ == typ.Elem() {
				return true
			}
		}
	}

	return false
}

func getOmitEmpty(ic *Inception, sf *StructField) string {
	switch sf.Typ.Kind() {

	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return "if len(mj." + sf.Name + ") != 0 {" + "\n"

	case reflect.Int,
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
		reflect.Float64:
		return "if mj." + sf.Name + " != 0 {" + "\n"

	case reflect.Bool:
		return "if mj." + sf.Name + " != false {" + "\n"

	case reflect.Interface, reflect.Ptr:
		return "if mj." + sf.Name + " != nil {" + "\n"

	default:
		// TODO(pquerna): fix types
		return "if true {" + "\n"
	}
}

func getGetInnerValue(ic *Inception, name string, typ reflect.Type, ptr bool) string {
	var out = ""

	if typ.Implements(marshalerBufType) ||
		typeInInception(ic, typ) ||
		typ.Implements(marshalerType) ||
		reflect.PtrTo(typ).Implements(marshalerType) {

		out += tplStr(encodeTpl["handleMarshaler"], handleMarshaler{
			IC:             ic,
			Name:           name,
			MarshalJSONBuf: typ.Implements(marshalerBufType) || typeInInception(ic, typ),
			Marshaler:      typ.Implements(marshalerType) || reflect.PtrTo(typ).Implements(marshalerType),
		})
		return out
	}

	ptname := name
	if ptr {
		ptname = "*" + name
	}

	switch typ.Kind() {
	case reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64:
		ic.OutputImports[`fflib "github.com/pquerna/ffjson/fflib/v1"`] = true
		out += "fflib.FormatBits(buf, uint64(" + ptname + "), 10, " + ptname + " < 0)" + "\n"
	case reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64,
		reflect.Uintptr:
		ic.OutputImports[`fflib "github.com/pquerna/ffjson/fflib/v1"`] = true
		out += "fflib.FormatBits(buf, uint64(" + ptname + "), 10, false)" + "\n"
	case reflect.Float32:
		ic.OutputImports[`"strconv"`] = true
		out += "buf.Write(strconv.AppendFloat([]byte{}, float64(" + ptname + "), 'f', 10, 32))" + "\n"
	case reflect.Float64:
		ic.OutputImports[`"strconv"`] = true
		out += "buf.Write(strconv.AppendFloat([]byte{}, " + ptname + ", 'f', 10, 64))" + "\n"
	case reflect.Array,
		reflect.Slice:
		out += "if " + name + "!= nil {" + "\n"
		out += "buf.WriteString(`[`)" + "\n"
		out += "for i, v := range " + name + "{" + "\n"
		out += "if i != 0 {" + "\n"
		out += "buf.WriteString(`,`)" + "\n"
		out += "}" + "\n"
		out += getGetInnerValue(ic, "v", typ.Elem(), false)
		out += "}" + "\n"
		out += "buf.WriteString(`]`)" + "\n"
		out += "} else {" + "\n"
		out += "buf.WriteString(`null`)" + "\n"
		out += "}" + "\n"
	case reflect.String:
		ic.OutputImports[`fflib "github.com/pquerna/ffjson/fflib/v1"`] = true
		out += "fflib.WriteJsonString(buf, " + ptname + ")" + "\n"
	case reflect.Ptr:
		out += "if " + name + "!= nil {" + "\n"
		switch typ.Elem().Kind() {
		case reflect.Struct:
			out += getGetInnerValue(ic, name, typ.Elem(), false)
		default:
			out += getGetInnerValue(ic, "*"+name, typ.Elem(), false)
		}
		out += "} else {" + "\n"
		out += "buf.WriteString(`null`)" + "\n"
		out += "}" + "\n"
	case reflect.Bool:
		out += "if " + ptname + " {" + "\n"
		out += "buf.WriteString(`true`)" + "\n"
		out += "} else {" + "\n"
		out += "buf.WriteString(`false`)" + "\n"
		out += "}" + "\n"
	case reflect.Interface:
		ic.OutputImports[`"encoding/json"`] = true
		out += fmt.Sprintf("/* Interface types must use runtime reflection. type=%v kind=%v */\n", typ, typ.Kind())
		out += "obj, err = json.Marshal(" + name + ")" + "\n"
		out += "if err != nil {" + "\n"
		out += "  return err" + "\n"
		out += "}" + "\n"
		out += "buf.Write(obj)" + "\n"
	default:
		ic.OutputImports[`"encoding/json"`] = true
		out += fmt.Sprintf("/* Falling back. type=%v kind=%v */\n", typ, typ.Kind())
		out += "obj, err = json.Marshal(" + name + ")" + "\n"
		out += "if err != nil {" + "\n"
		out += "  return err" + "\n"
		out += "}" + "\n"
		out += "buf.Write(obj)" + "\n"
	}

	return out
}

func getValue(ic *Inception, sf *StructField) string {
	return getGetInnerValue(ic, "mj."+sf.Name, sf.Typ, sf.Pointer)
}

func p2(v uint32) uint32 {
	v--
	v |= v >> 1
	v |= v >> 2
	v |= v >> 4
	v |= v >> 8
	v |= v >> 16
	v++
	return v
}

func getTotalSize(si *StructInfo) uintptr {
	rv := si.Typ.Size()
	for _, f := range si.Fields {
		rv += f.Typ.Size()
	}
	return rv
}

func getBufGrowSize(si *StructInfo) uint32 {

	// TOOD(pquerna): automatically calc a better grow size based on history
	// of a struct.
	return p2(uint32(float32(getTotalSize(si)) * 3.0))
}

func CreateMarshalJSON(ic *Inception, si *StructInfo) error {
	conditionalWrites := false
	out := ""

	ic.OutputImports[`"bytes"`] = true

	out += `func (mj *` + si.Name + `) MarshalJSON() ([]byte, error) {` + "\n"
	out += `var buf fflib.Buffer` + "\n"

	out += fmt.Sprintf("buf.Grow(%d)\n", getBufGrowSize(si))
	out += `err := mj.MarshalJSONBuf(&buf)` + "\n"
	out += `if err != nil {` + "\n"
	out += "  return nil, err" + "\n"
	out += `}` + "\n"
	out += `return buf.Bytes(), nil` + "\n"
	out += `}` + "\n"

	for _, f := range si.Fields {
		if f.OmitEmpty || f.Pointer {
			// if we have >= 1 non-conditional write, we can
			// assume our trailing logic is reaosnable.
			if conditionalWrites {
				conditionalWrites = false
				break
			}
			conditionalWrites = true
		}
	}

	out += `func (mj *` + si.Name + `) MarshalJSONBuf(buf fflib.EncodingBuffer) (error) {` + "\n"
	out += `var err error` + "\n"
	out += `var obj []byte` + "\n"
	if conditionalWrites {
		out += `var wroteAnyFields bool = false` + "\n"
	}

	out += `_ = obj` + "\n"
	out += `_ = err` + "\n"
	out += "buf.WriteString(`{`)" + "\n"

	for _, f := range si.Fields {
		if f.OmitEmpty {
			out += getOmitEmpty(ic, f)
		}

		if f.Pointer {
			out += "if mj." + f.Name + " != nil {" + "\n"
		}

		if conditionalWrites {
			out += `wroteAnyFields = true` + "\n"
		}

		// JsonName is already escaped and quoted.
		out += "buf.WriteString(`" + f.JsonName + ":`)" + "\n"
		out += getValue(ic, f)
		out += "buf.WriteString(`, `)" + "\n"

		if f.Pointer {
			out += "}" + "\n"
		}

		if f.OmitEmpty {
			out += "}" + "\n"
		}
	}

	if conditionalWrites {
		out += `if wroteAnyFields {` + "\n"
		out += "  	buf.Rewind(2)" + "\n"
		out += "	buf.WriteByte('}')" + "\n"
		out += `} else {` + "\n"
		out += "	buf.WriteByte('}')" + "\n"
		out += `}` + "\n"
	} else {
		out += "buf.Rewind(2)" + "\n"
		out += "buf.WriteByte('}')" + "\n"
	}

	out += `return nil` + "\n"
	out += `}` + "\n"
	ic.OutputFuncs = append(ic.OutputFuncs, out)
	return nil
}
