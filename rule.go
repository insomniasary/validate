package validate

import (
	"strings"
)

// Rules definition
type Rules []*Rule

/*************************************************************
 * validation rule
 *************************************************************/

// Rule definition
type Rule struct {
	// eg "create" "update"
	scene string
	// need validate fields. allow multi.
	fields []string
	// is optional, only validate on value is not empty. sometimes
	optional bool
	// skip validate not exist field/empty value
	skipEmpty bool
	// default value setting
	defValue interface{}
	// error message
	message string
	// error messages, if fields contains multi field.
	// eg {
	// 	"field": "error message",
	// 	"field.validator": "error message",
	// }
	messages map[string]string
	// the input validator name
	validator string
	// the real validator name
	realName string
	// real validator name is requiredXXX validators
	nameNotRequired bool
	// arguments for the validator
	arguments []interface{}
	// --- some hooks function
	// has beforeFunc. if return false, skip validate current rule
	beforeFunc func(v *Validation) bool // func (val interface{}) bool
	// you can custom filter func
	filterFunc func(val interface{}) (interface{}, error)
	// custom check func's mate info
	checkFuncMeta *funcMeta
	// custom check is empty.
	emptyChecker func(val interface{}) bool
}

// NewRule create new Rule instance
func NewRule(fields, validator string, args ...interface{}) *Rule {
	return &Rule{
		fields: stringSplit(fields, ","),
		// validator args
		arguments: args,
		validator: validator,
	}
}

// SetScene name for the rule.
func (r *Rule) SetScene(scene string) *Rule {
	r.scene = scene
	return r
}

// SetOptional only validate on value is not empty.
func (r *Rule) SetOptional(optional bool) {
	r.optional = optional
}

// SetSkipEmpty skip validate not exist field/empty value
func (r *Rule) SetSkipEmpty(skipEmpty bool) {
	r.skipEmpty = skipEmpty
}

// SetDefValue for the rule
// func (r *Rule) SetDefValue(defValue interface{}) {
// 	r.defValue = defValue
// }

// SetCheckFunc set custom validate func.
func (r *Rule) SetCheckFunc(checkFunc interface{}) *Rule {
	var name string
	if r.validator != "" {
		name = "rule_" + r.validator
	} else {
		name = "rule_" + strings.Join(r.fields, "_")
	}

	fv := checkValidatorFunc(name, checkFunc)
	r.checkFuncMeta = newFuncMeta(name, false, fv)
	return r
}

// SetFilterFunc for the rule
func (r *Rule) SetFilterFunc(fn func(val interface{}) (interface{}, error)) *Rule {
	r.filterFunc = fn
	return r
}

// SetBeforeFunc for the rule. will call it before validate.
func (r *Rule) SetBeforeFunc(fn func(v *Validation) bool) {
	r.beforeFunc = fn
}

// SetMessage set error message.
// Usage:
// 	v.AddRule("name", "required").SetMessage("error message")
func (r *Rule) SetMessage(errMsg string) *Rule {
	r.message = errMsg
	return r
}

// SetMessages set error message map.
// Usage:
// 	v.AddRule("name,email", "required").SetMessages(MS{
// 		"name": "error message 1",
// 		"email": "error message 2",
// 	})
func (r *Rule) SetMessages(msgMap MS) *Rule {
	r.messages = msgMap
	return r
}

// Fields field names list
func (r *Rule) Fields() []string {
	return r.fields
}

func (r *Rule) errorMessage(field, validator string, v *Validation) (msg string) {
	if r.messages != nil {
		var ok bool
		// use full key. "field.validator"
		fKey := field + "." + validator
		if msg, ok = r.messages[fKey]; ok {
			return
		}

		if msg, ok = r.messages[field]; ok {
			return
		}
	}

	if r.message != "" {
		return r.message
	}

	// built in error messages
	return v.trans.Message(validator, field, r.arguments...)
}

/*************************************************************
 * add validate rules
 *************************************************************/

// StringRule add field rules by string
//
// Usage:
// 	v.StringRule("name", "required|string|minLen:6")
// 	// will try convert to int before apply validate.
// 	v.StringRule("age", "required|int|min:12", "toInt")
func (v *Validation) StringRule(field, rule string, filterRule ...string) *Validation {
	rule = strings.TrimSpace(rule)
	rules := stringSplit(strings.Trim(rule, "|:"), "|")
	for _, validator := range rules {
		validator = strings.Trim(validator, ":")
		if validator == "" { // empty
			continue
		}

		// has args "min:12"
		if strings.ContainsRune(validator, ':') {
			list := stringSplit(validator, ":")
			// reassign value
			validator := list[0]
			realName := ValidatorName(validator)
			switch realName {
			// set error message for the field TODO
			// case "message":
			// 	// message key like "age.required"
			// 	v.trans.AddMessage(field+"."+validator, list[1])
			// add default value for the field
			case "default":
				v.SetDefValue(field, list[1])
			// eg 'regex:\d{4,6}' dont need split args. args is "\d{4,6}"
			case "regexp":
				v.AddRule(field, validator, list[1])
			// some special validator. need merge args to one.
			case "enum", "notIn":
				v.AddRule(field, validator, parseArgString(list[1]))
			default:
				args := parseArgString(list[1])
				v.AddRule(field, validator, strings2Args(args)...)
			}
		} else {
			v.AddRule(field, validator)
		}
	}

	if len(filterRule) > 0 {
		v.FilterRule(field, filterRule[0])
	}
	return v
}

// StringRules add multi rules by string map.
// Usage:
// 	v.StringRules(map[string]string{
// 		"name": "required|string|min_len:12",
// 		"age": "required|int|min:12",
// 	})
func (v *Validation) StringRules(mp MS) *Validation {
	for name, rule := range mp {
		v.StringRule(name, rule)
	}
	return v
}

// ConfigRules add multi rules by string map. alias of StringRules()
// Usage:
// 	v.ConfigRules(map[string]string{
// 		"name": "required|string|min:12",
// 		"age": "required|int|min:12",
// 	})
func (v *Validation) ConfigRules(mp MS) *Validation {
	for name, rule := range mp {
		v.StringRule(name, rule)
	}
	return v
}

// AddRule for current validation
func (v *Validation) AddRule(fields, validator string, args ...interface{}) *Rule {
	return v.addOneRule(fields, validator, ValidatorName(validator), args)
}

// add one Rule for current validation
func (v *Validation) addOneRule(fields, validator, realName string, args []interface{}) *Rule {
	rule := NewRule(fields, validator, args...)

	// init some settings
	rule.realName = realName
	rule.skipEmpty = v.SkipOnEmpty
	// validator name is not "required"
	rule.nameNotRequired = !strings.HasPrefix(realName, "required")

	// append
	v.rules = append(v.rules, rule)
	return rule
}

// AppendRule instance
func (v *Validation) AppendRule(rule *Rule) *Rule {
	rule.realName = ValidatorName(rule.validator)
	rule.skipEmpty = v.SkipOnEmpty
	// validator name is not "required"
	rule.nameNotRequired = !strings.HasPrefix(rule.realName, "required")

	// append
	v.rules = append(v.rules, rule)
	return rule
}
