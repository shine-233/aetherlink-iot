// 文件用途：计算字段表达式求值器的单元测试。
// 核心逻辑：覆盖四则运算、括号、变量替换、除零、未解析占位符等边界。
package service

import (
	"testing"
)

func TestEvalArithmetic(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		want    float64
		wantErr bool
	}{
		{name: "simple add", expr: "1+2", want: 3},
		{name: "simple mul", expr: "3*4", want: 12},
		{name: "division", expr: "10/4", want: 2.5},
		{name: "precedence", expr: "2+3*4", want: 14},
		{name: "parentheses", expr: "(2+3)*4", want: 20},
		{name: "negative", expr: "-5+3", want: -2},
		{name: "decimal", expr: "3.5*2", want: 7},
		{name: "complex", expr: "(25-10)*2/5+1", want: 7},
		{name: "div by zero", expr: "5/0", wantErr: true},
		{name: "invalid char", expr: "abc", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := evalArithmetic(tt.expr)
			if (err != nil) != tt.wantErr {
				t.Fatalf("evalArithmetic(%q) error = %v, wantErr %v", tt.expr, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Fatalf("evalArithmetic(%q) = %v, want %v", tt.expr, got, tt.want)
			}
		})
	}
}

func TestSubstituteAndEvaluate(t *testing.T) {
	data := map[string]interface{}{
		"temperature": 25.0,
		"voltage":     220.0,
		"current":     5.0,
		"humidity":    60,
	}
	tests := []struct {
		name    string
		expr    string
		want    float64
		wantErr bool
	}{
		{name: "power calc", expr: "{voltage} * {current}", want: 1100},
		{name: "fahrenheit", expr: "{temperature} * 1.8 + 32", want: 77},
		{name: "avg with parens", expr: "({temperature} + 30) / 2", want: 27.5},
		{name: "int value", expr: "{humidity} * 1.5", want: 90},
		{name: "unresolved placeholder", expr: "{missing_key} * 2", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := substituteAndEvaluate(tt.expr, data)
			if (err != nil) != tt.wantErr {
				t.Fatalf("substituteAndEvaluate(%q) error = %v, wantErr %v", tt.expr, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Fatalf("substituteAndEvaluate(%q) = %v, want %v", tt.expr, got, tt.want)
			}
		})
	}
}
