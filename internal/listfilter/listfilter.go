package listfilter

import (
	"fmt"
	"path"
	"strings"

	"github.com/daulet/k11s/internal/protocol"
)

type Operator string

const (
	OperatorExact   Operator = "="
	OperatorPattern Operator = "~"
)

type Predicate struct {
	Field    string
	Operator Operator
	Value    string
}

func (p Predicate) String() string {
	return p.Field + string(p.Operator) + p.Value
}

func Normalize(raw string) (string, error) {
	predicate, active, err := Parse(raw)
	if err != nil || !active {
		return "", err
	}
	return predicate.String(), nil
}

func Parse(raw string) (Predicate, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Predicate{}, false, nil
	}

	operatorAt := strings.IndexAny(raw, "=~")
	if operatorAt <= 0 {
		return Predicate{}, false, fmt.Errorf("filter must use field=value or field~pattern")
	}
	if operatorAt == len(raw)-1 {
		return Predicate{}, false, fmt.Errorf("filter value is required")
	}

	field := strings.ToLower(strings.TrimSpace(raw[:operatorAt]))
	value := strings.TrimSpace(raw[operatorAt+1:])
	if field == "" {
		return Predicate{}, false, fmt.Errorf("filter field is required")
	}
	if value == "" {
		return Predicate{}, false, fmt.Errorf("filter value is required")
	}
	if strings.ContainsAny(value, " \t\r\n") {
		return Predicate{}, false, fmt.Errorf("filter value cannot contain whitespace")
	}

	operator := Operator(raw[operatorAt : operatorAt+1])
	switch operator {
	case OperatorExact, OperatorPattern:
	default:
		return Predicate{}, false, fmt.Errorf("unsupported filter operator %q", operator)
	}

	switch field {
	case "node":
	default:
		return Predicate{}, false, fmt.Errorf("unsupported filter field %q", field)
	}

	if operator == OperatorPattern && hasGlobMeta(value) {
		if _, err := path.Match(strings.ToLower(value), ""); err != nil {
			return Predicate{}, false, fmt.Errorf("invalid node pattern %q: %w", value, err)
		}
	}

	return Predicate{
		Field:    field,
		Operator: operator,
		Value:    value,
	}, true, nil
}

func AppliesToResource(resource string, raw string) bool {
	predicate, active, err := Parse(raw)
	if err != nil || !active {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(resource), "pods") && predicate.Field == "node"
}

func FilterItems(resource string, items []protocol.ResourceItem, raw string) []protocol.ResourceItem {
	predicate, active, err := Parse(raw)
	if err != nil || !active || !strings.EqualFold(strings.TrimSpace(resource), "pods") {
		return append([]protocol.ResourceItem(nil), items...)
	}

	filtered := make([]protocol.ResourceItem, 0, len(items))
	for _, item := range items {
		if predicate.matchesNode(item.Node) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func (p Predicate) matchesNode(node string) bool {
	node = strings.TrimSpace(node)
	value := strings.TrimSpace(p.Value)

	switch p.Operator {
	case OperatorExact:
		return node == value
	case OperatorPattern:
		nodeLower := strings.ToLower(node)
		valueLower := strings.ToLower(value)
		if !hasGlobMeta(valueLower) {
			return strings.Contains(nodeLower, valueLower)
		}
		matched, err := path.Match(valueLower, nodeLower)
		return err == nil && matched
	default:
		return false
	}
}

func hasGlobMeta(value string) bool {
	return strings.ContainsAny(value, "*?[")
}
