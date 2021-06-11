// Copyright (c) 2017-2020 The Elastos Foundation
// Use of this source code is governed by an MIT
// license that can be found in the LICENSE file.
//

package common

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/elastos/Elastos.ELA/common"
	"golang.org/x/crypto/ripemd160"
)

func ToCodeHash(code []byte) *common.Uint160 {
	hash := sha256.Sum256(code)
	md160 := ripemd160.New()
	md160.Write(hash[:])
	sum := common.Uint160{}
	copy(sum[:], md160.Sum(nil))
	return &sum
}

func ToProgramHash(prefix byte, code []byte) *common.Uint168 {
	hash := sha256.Sum256(code)
	md160 := ripemd160.New()
	md160.Write(hash[:])
	sum := common.Uint168{}
	copy(sum[:], md160.Sum([]byte{prefix}))
	return &sum
}

func SortProgramHashByCodeHash(hashes []common.Uint168) {
	sort.Slice(hashes, func(i, j int) bool {
		return hashes[i].ToCodeHash().Compare(hashes[j].ToCodeHash()) < 0
	})
}

func BytesReverse(u []byte) []byte {
	for i, j := 0, len(u)-1; i < j; i, j = i+1, j-1 {
		u[i], u[j] = u[j], u[i]
	}
	return u
}

func BytesToHexString(data []byte) string {
	return hex.EncodeToString(data)
}

func HexStringToBytes(value string) ([]byte, error) {
	return hex.DecodeString(value)
}

func ToReversedString(hash common.Uint256) string {
	return BytesToHexString(BytesReverse(hash[:]))
}

func FromReversedString(reversed string) ([]byte, error) {
	bytes, err := HexStringToBytes(reversed)
	return BytesReverse(bytes), err
}

func IntToBytes(n int) []byte {
	tmp := int32(n)
	bytesBuffer := bytes.NewBuffer([]byte{})
	binary.Write(bytesBuffer, binary.LittleEndian, tmp)
	return bytesBuffer.Bytes()
}

func BytesToInt16(b []byte) int16 {
	bytesBuffer := bytes.NewBuffer(b)
	var tmp int16
	binary.Read(bytesBuffer, binary.BigEndian, &tmp)
	return int16(tmp)
}

func ClearBytes(arr []byte) {
	for i := 0; i < len(arr); i++ {
		arr[i] = 0
	}
}

func ReverseHexString(s string) (string, error) {
	b, err := HexStringToBytes(s)
	if err != nil {
		return "", err
	}
	for i := 0; i < len(b)/2; i++ {
		b[i], b[len(b)-i-1] = b[len(b)-i-1], b[i]
	}
	return BytesToHexString(b), nil
}

//func ContainsU168(c common.Uint168, s []common.Uint168) bool {
//	for _, v := range s {
//		if v.IsEqual(c) {
//			return true
//		}
//	}
//	return false
//}

//Map2Struct convert map into struct
func Map2Struct(src map[string]interface{}, destStrct interface{}) {
	value := reflect.ValueOf(destStrct)
	e := value.Elem()
	for k, v := range src {
		f := e.FieldByName(strings.ToUpper(k[:1]) + k[1:])
		if !f.IsValid() {
			continue
		}
		if !f.CanSet() {
			continue
		}
		mv := reflect.ValueOf(v)
		mvt := mv.Kind().String()
		sft := f.Kind().String()
		if sft != mvt {
			if mvt == "string" && (strings.Index(sft, "int") != -1) {
				if sft == "int64" {
					i, err := strconv.ParseInt(v.(string), 10, 64)
					if err == nil {
						f.Set(reflect.ValueOf(i))
					}
				} else if sft == "int32" {
					i, err := strconv.ParseInt(v.(string), 10, 32)
					r := int32(i)
					if err == nil {
						f.Set(reflect.ValueOf(r))
					}
				} else if sft == "int" {
					i, err := strconv.Atoi(v.(string))
					if err == nil {
						f.Set(reflect.ValueOf(i))
					}
				} else if sft == "uint64" {
					i, err := strconv.ParseUint(v.(string), 10, 64)
					if err == nil {
						f.Set(reflect.ValueOf(i))
					}
				} else if sft == "uint32" {
					i, err := strconv.ParseUint(v.(string), 10, 32)
					r := uint32(i)
					if err == nil {
						f.Set(reflect.ValueOf(r))
					}
				} else if sft == "uint" {
					i, err := strconv.ParseUint(v.(string), 10, 0)
					r := uint(i)
					if err == nil {
						f.Set(reflect.ValueOf(r))
					}
				}
			}

			if mvt == "int" && (strings.Index(sft, "int") != -1) {
				if sft == "int64" {
					r := int64(v.(int))
					f.Set(reflect.ValueOf(r))
				} else if sft == "int32" {
					r := int32(v.(int))
					f.Set(reflect.ValueOf(r))
				} else if sft == "uint64" {
					r := uint64(v.(int))
					f.Set(reflect.ValueOf(r))
				} else if sft == "uint32" {
					r := uint32(v.(int))
					f.Set(reflect.ValueOf(r))
				} else if sft == "uint" {
					r := uint(v.(int))
					f.Set(reflect.ValueOf(r))
				}
			}

			// make string and string[] more friendly
			if mvt == "string" && sft == "slice" {
				_, ok := f.Interface().([]string)
				if ok {
					f.Set(reflect.ValueOf(strings.Split(v.(string), ",")))
				}
			}

			// make float64 and int64 more friendly
			if mvt == "float64" && sft == "int64" {
				f.Set(reflect.ValueOf(int64(v.(float64))))
			}
			continue
		}
		f.Set(mv)
	}
}

func Sha256D(data []byte) [32]byte {
	once := sha256.Sum256(data)
	return sha256.Sum256(once[:])
}

func Hash(data []byte) common.Uint256 {
	return common.Uint256(Sha256D(data))
}

// Goid returns the current goroutine id.
func Goid() string {
	var buf [32]byte
	n := runtime.Stack(buf[:], false)
	fields := strings.Fields(string(buf[:n]))
	if len(fields) <= 1 {
		return ""
	}
	return fields[1]
}

// VarIntSerializeSize returns the number of bytes it would take to serialize
// val as a variable length integer.
func VarIntSerializeSize(val uint64) int {
	// The value is small enough to be represented by itself, so it's
	// just 1 byte.
	if val < 0xfd {
		return 1
	}

	// Discriminant 1 byte plus 2 bytes for the uint16.
	if val <= math.MaxUint16 {
		return 3
	}

	// Discriminant 1 byte plus 4 bytes for the uint32.
	if val <= math.MaxUint32 {
		return 5
	}

	// Discriminant 1 byte plus 8 bytes for the uint64.
	return 9
}
