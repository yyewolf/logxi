package log

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"runtime/debug"
	"strconv"
	"time"
)

// JSONFormatter formats log entries as JSON. This should be used
// in production because it is machine parseable.
type JSONFormatter struct {
	name string
}

// NewJSONFormatter creates a new instance of JSONFormatter.
func NewJSONFormatter(name string) *JSONFormatter {
	return &JSONFormatter{name: name}
}

func (jf *JSONFormatter) writeString(buf *bytes.Buffer, s string) {
	b, err := json.Marshal(s)
	if err != nil {
		InternalLog.Error("Could not json.Marshal string.", "str", s)
		buf.WriteString(`"Could not marshal this key's string"`)
		return
	}
	buf.Write(b)
}

func (jf *JSONFormatter) writeError(buf *bytes.Buffer, err error) {
	jf.writeString(buf, err.Error())
	jf.set(buf, callstackKey, string(debug.Stack()))
	return
}

func (jf *JSONFormatter) appendValue(buf *bytes.Buffer, val interface{}) {
	if val == nil {
		buf.WriteString("null")
		return
	}

	value := reflect.ValueOf(val)
	kind := value.Kind()
	if kind == reflect.Ptr {
		value = value.Elem()
		kind = value.Kind()
	}
	switch kind {
	case reflect.Bool:
		if value.Bool() {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		buf.WriteString(strconv.FormatInt(value.Int(), 10))

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		buf.WriteString(strconv.FormatUint(value.Uint(), 10))

	case reflect.Float32:
		buf.WriteString(strconv.FormatFloat(value.Float(), 'g', -1, 32))

	case reflect.Float64:
		buf.WriteString(strconv.FormatFloat(value.Float(), 'g', -1, 64))

	default:
		if err, ok := val.(error); ok {
			jf.writeError(buf, err)
			return
		}

		b, err := json.Marshal(value.Interface())
		if err != nil {
			InternalLog.Error("Could not json.Marshal value: ", "formatter", "JSONFormatter", "err", err.Error())
			// must always log, use sprintf to get a string
			s := fmt.Sprintf("%#v", value.Interface())
			b, err = json.Marshal(s)
			if err != nil {
				// should never get here, but JSONFormatter should never panic
				msg := "Could not Sprintf value"
				InternalLog.Error(msg)
				buf.WriteString(`"` + msg + `"`)
				return
			}
		}
		buf.Write(b)
	}
}

func (jf *JSONFormatter) set(buf *bytes.Buffer, key string, val interface{}) {
	// WARNING: assumes this is not first key
	buf.WriteString(`, "`)
	buf.WriteString(key)
	buf.WriteString(`":`)
	jf.appendValue(buf, val)
}

// Format formats log entry as JSON.
func (jf *JSONFormatter) Format(buf *bytes.Buffer, level int, msg string, args []interface{}) {
	buf.WriteString(`{"_t":"`)
	buf.WriteString(time.Now().Format(timeFormat))
	buf.WriteRune('"')

	buf.WriteString(`, "_l":"`)
	buf.WriteString(LevelMap[level])
	buf.WriteRune('"')

	buf.WriteString(`, "_n":"`)
	buf.WriteString(jf.name)
	buf.WriteRune('"')

	buf.WriteString(`, "_m":`)
	jf.appendValue(buf, msg)

	var lenArgs = len(args)
	if lenArgs > 0 {
		if lenArgs%2 == 0 {
			for i := 0; i < lenArgs; i += 2 {
				if key, ok := args[i].(string); ok {
					if key == "" {
						// show key is invalid
						jf.set(buf, badKeyAtIndex(i), args[i+1])
					} else {
						jf.set(buf, key, args[i+1])
					}
				} else {
					// show key is invalid
					jf.set(buf, badKeyAtIndex(i), args[i+1])
				}
			}
		} else {
			jf.set(buf, warnImbalancedKey, args)
		}
	}
	buf.WriteString("}\n")
}

// LogEntry returns the JSON log entry object built by Format(). Used by
// HappyDevFormatter to ensure any data logged while developing will properly
// log in production.
func (jf *JSONFormatter) LogEntry(level int, msg string, args []interface{}) map[string]interface{} {
	var buf bytes.Buffer
	jf.Format(&buf, level, msg, args)
	var entry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &entry)
	if err != nil {
		panic("Unable to unmarhsal entry from JSONFormatter: " + err.Error())
	}
	return entry
}
