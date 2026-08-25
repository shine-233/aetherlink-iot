// 文件用途：计算字段表达式求值器，安全地计算含变量的算术表达式。
// 核心逻辑：将 {key} 占位符替换为实际值后，用递归下降解析器求值。
// 关键注意事项：仅支持四则运算和括号，不支持函数调用，防止代码注入。
package service

import (
	"fmt"
	"strconv"
	"strings"
)

// evalArithmetic 安全计算不含变量引用的纯数字算术表达式。
// 支持 +, -, *, / 和括号，使用递归下降解析器（无外部依赖）。
func evalArithmetic(expr string) (float64, error) {
	p := &arithParser{input: strings.ReplaceAll(expr, " ", "")}
	p.pos = 0
	result, err := p.parseExpression()
	if err != nil {
		return 0, err
	}
	if p.pos < len(p.input) {
		return 0, fmt.Errorf("unexpected character at position %d: %c", p.pos, p.input[p.pos])
	}
	return result, nil
}

type arithParser struct {
	input string
	pos   int
}

func (p *arithParser) peek() byte {
	if p.pos < len(p.input) {
		return p.input[p.pos]
	}
	return 0
}

func (p *arithParser) parseExpression() (float64, error) {
	left, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for p.pos < len(p.input) {
		op := p.peek()
		if op != '+' && op != '-' {
			break
		}
		p.pos++
		right, err := p.parseTerm()
		if err != nil {
			return 0, err
		}
		if op == '+' {
			left += right
		} else {
			left -= right
		}
	}
	return left, nil
}

func (p *arithParser) parseTerm() (float64, error) {
	left, err := p.parseFactor()
	if err != nil {
		return 0, err
	}
	for p.pos < len(p.input) {
		op := p.peek()
		if op != '*' && op != '/' {
			break
		}
		p.pos++
		right, err := p.parseFactor()
		if err != nil {
			return 0, err
		}
		if op == '*' {
			left *= right
		} else {
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			left /= right
		}
	}
	return left, nil
}

func (p *arithParser) parseFactor() (float64, error) {
	ch := p.peek()
	if ch == '(' {
		p.pos++
		result, err := p.parseExpression()
		if err != nil {
			return 0, err
		}
		if p.peek() != ')' {
			return 0, fmt.Errorf("expected closing parenthesis")
		}
		p.pos++
		return result, nil
	}
	if ch == '-' {
		p.pos++
		val, err := p.parseFactor()
		return -val, err
	}
	if ch == '+' {
		p.pos++
		return p.parseFactor()
	}
	// 解析数字
	start := p.pos
	for p.pos < len(p.input) && (p.input[p.pos] >= '0' && p.input[p.pos] <= '9' || p.input[p.pos] == '.') {
		p.pos++
	}
	if start == p.pos {
		return 0, fmt.Errorf("expected number at position %d", p.pos)
	}
	return strconv.ParseFloat(p.input[start:p.pos], 64)
}

// substituteAndEvaluate 将表达式中的 {key} 占位符替换为实际值后求值。
func substituteAndEvaluate(expression string, data map[string]interface{}) (float64, error) {
	result := expression
	for key, val := range data {
		placeholder := "{" + key + "}"
		numStr := ""
		switch v := val.(type) {
		case float64:
			numStr = strconv.FormatFloat(v, 'f', -1, 64)
		case int:
			numStr = strconv.Itoa(v)
		case int64:
			numStr = strconv.FormatInt(v, 10)
		case string:
			numStr = v
		default:
			continue
		}
		result = strings.ReplaceAll(result, placeholder, numStr)
	}
	// 检查是否还有未替换的占位符
	if strings.Contains(result, "{") || strings.Contains(result, "}") {
		return 0, fmt.Errorf("expression contains unresolved placeholders: %s", result)
	}
	return evalArithmetic(result)
}
