package obfuscator

import (
	"context"
	"encoding/base64"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/LazarenkoA/1c-language-parser/ast"
	"github.com/knetic/govaluate"
	"github.com/pkg/errors"
)

type Config struct {
	// RepExpByTernary заменять выражение тернарными операторами
	RepExpByTernary bool

	// RepLoopByGoto заменять циклы на Перейти
	RepLoopByGoto bool

	// RepExpByInvoke прятать выражения в Выполнить() Вычислить()
	RepExpByInvoke bool

	// HideString прятать строки
	HideString bool

	// ChangeConditions изменять условия
	ChangeConditions bool

	// AppendGarbage добавлять мусора
	AppendGarbage bool

	// ShuffleExpressions изменять порядок выражений
	// ShuffleExpressions bool
}

type Obfuscator struct {
	ctx                  context.Context
	conf                 Config
	a                    *ast.AstNode
	rand                 *rand.Rand
	trueCondition        chan string
	falseCondition       chan string
	decodeStringFuncName map[string]string
}

func init() {

}

func NewObfuscatory(ctx context.Context, conf Config) *Obfuscator {
	c := &Obfuscator{
		ctx:                  ctx,
		conf:                 conf,
		rand:                 rand.New(rand.NewSource(time.Now().UnixNano())),
		trueCondition:        make(chan string, 10),
		falseCondition:       make(chan string, 10),
		decodeStringFuncName: make(map[string]string),
	}

	c.genCondition()
	return c
}

func (c *Obfuscator) Obfuscate(code string) (string, error) {
	c.a = ast.NewAST(code)
	if err := c.a.Parse(); err != nil {
		return "", err
	}

	if len(c.a.ModuleStatement.Body) == 0 {
		return code, nil
	}

	// pp.Println(c.a.ModuleStatement)

	c.a.ModuleStatement.Walk(c.walkStep)

	result := c.a.Print(ast.PrintConf{OneLine: true})
	// result = strings.ToLower(result) // нельзя так делать, все поломает
	return result, nil
}

func (c *Obfuscator) walkStep(current *ast.FunctionOrProcedure, item *ast.Statement) {
	key := float64(c.rand.Intn(90) + 10)

	switch v := (*item).(type) {
	case *ast.IfStatement:
		v.Expression = c.appendConditions(v.Expression)
		if c.conf.ChangeConditions {
			c.appendIfElseBlock(&v.IfElseBlock, c.rand.Intn(5))
			c.appendGarbage(&v.ElseBlock)
			c.appendGarbage(&v.TrueBlock)
		}
		// v.TrueBlock = c.shuffleExpressions(v.TrueBlock)
		// v.ElseBlock = c.shuffleExpressions(v.ElseBlock)
	case *ast.FunctionOrProcedure:
		c.appendGarbage(&v.Body)
		// v.Body = c.shuffleExpressions(v.Body)
	case ast.MethodStatement:
		for i, param := range v.Param {
			if exp, ok := param.(*ast.ExpStatement); ok {
				tmp := ast.Statement(exp)
				c.walkStep(current, &tmp)
			}
			if str, ok := param.(string); ok {
				v.Param[i] = ast.MethodStatement{
					Name:  c.decodeStringFunc(current.Directive),
					Param: []ast.Statement{c.obfuscateString(str, int32(key)), c.hideValue(key, 4, false)},
				}
			}
		}
	case *ast.ReturnStatement:
		if str, ok := v.Param.(string); ok && c.conf.HideString {
			v.Param = ast.MethodStatement{
				Name:  c.decodeStringFunc(current.Directive),
				Param: []ast.Statement{c.obfuscateString(str, int32(key)), c.hideValue(key, 4, false)},
			}
		}
	case *ast.ExpStatement:
		switch r := v.Right.(type) {
		case string:
			if c.conf.HideString {
				v.Right = ast.MethodStatement{
					Name:  c.decodeStringFunc(current.Directive),
					Param: []ast.Statement{c.obfuscateString(r, int32(key)), c.hideValue(key, 4, false)},
				}
			}
			return
		case ast.NewObjectStatement:
			for i, param := range r.Param {
				if str, ok := param.(string); ok && c.conf.HideString {
					r.Param[i] = ast.MethodStatement{
						Name:  c.decodeStringFunc(current.Directive),
						Param: []ast.Statement{c.obfuscateString(str, int32(key)), c.hideValue(key, 4, false)},
					}
				}
			}
		}

		switch r := v.Left.(type) {
		case string:
			if c.conf.HideString {
				v.Left = ast.MethodStatement{
					Name:  c.decodeStringFunc(current.Directive),
					Param: []ast.Statement{c.obfuscateString(r, int32(key)), c.hideValue(key, 4, false)},
				}
			}

			return
		}

		if _, ok := v.Left.(ast.VarStatement); ok && c.conf.RepExpByInvoke {
			switch v.Right.(type) {
			case ast.MethodStatement, ast.CallChainStatement, ast.NewObjectStatement:
				str := c.a.PrintStatement(v.Right, ast.PrintConf{})
				if str[len(str)-1] == ';' {
					str = str[:len(str)-1]
				}

				v.Right = ast.MethodStatement{
					Name: "Вычислить",
					Param: []ast.Statement{ast.MethodStatement{
						Name:  c.decodeStringFunc(current.Directive),
						Param: []ast.Statement{c.obfuscateString(str, int32(key)), c.hideValue(key, 4, false)},
					}},
				}
			default:
				v.Right = c.hideValue(v.Right, 4, false)
			}
		}
	case ast.CallChainStatement:
		c.walkStep(current, &v.Unit)
	case *ast.LoopStatement:
		c.replaceLoopToGoto(&current.Body, v, false)
		// v.Body = c.shuffleExpressions(v.Body)
	}
}

func (c *Obfuscator) decodeStringFunc(directive string) string {
	if name, ok := c.decodeStringFuncName[directive]; ok {
		return name
	} else {
		name := c.newDecodeStringFunc(directive)
		c.decodeStringFuncName[directive] = name

		return name
	}
}

func (c *Obfuscator) hideValue(val interface{}, complexity int, force bool) ast.Statement {
	if !c.conf.RepExpByTernary && !force {
		return val
	}

	switch val.(type) {
	case string, bool, float64, int, time.Time, *ast.ExpStatement, ast.MethodStatement:
		return c.newTernary(val, c.rand.Intn(complexity-2)+2, c.rand.Intn(complexity-1))
	default:
		return val
	}
}

func (c *Obfuscator) appendGarbage(body *[]ast.Statement) {
	if !c.conf.AppendGarbage {
		return
	}

	if c.rand.Intn(2) == 1 {
		*body = append(*body, &ast.ExpStatement{
			Operation: ast.OpEq,
			Left:      ast.VarStatement{Name: c.randomString(20)},
			Right:     c.hideValue(c.randomString(5), 4, true),
		})
	}
	if c.rand.Intn(2) == 1 {
		*body = append(*body, &ast.ExpStatement{
			Operation: ast.OpEq,
			Left:      ast.VarStatement{Name: c.randomString(10)},
			Right:     c.hideValue(float64(c.rand.Intn(200)-100), 5, true),
		})
	}
	if c.rand.Intn(2) == 1 {
		IF := &ast.IfStatement{Expression: c.convStrExpToExpStatement(<-c.falseCondition)}

		if c.rand.Intn(2) == 1 {
			c.appendIfElseBlock(&IF.IfElseBlock, c.rand.Intn(5))
		}
		if c.rand.Intn(2) == 1 {
			c.appendGarbage(&IF.ElseBlock)
			c.appendGarbage(&IF.TrueBlock)
		}

		IF.TrueBlock = c.shuffleExpressions(IF.TrueBlock)
		IF.ElseBlock = c.shuffleExpressions(IF.ElseBlock)
		*body = append(*body, IF)
	}
	if c.rand.Intn(2) == 1 {
		loop := &ast.LoopStatement{WhileExpr: c.convStrExpToExpStatement(<-c.falseCondition)}
		if c.rand.Intn(2) == 1 {
			c.appendGarbage(&loop.Body)
		}

		loop.Body = c.shuffleExpressions(loop.Body)
		*body = append(*body, loop)
	}
}

func (c *Obfuscator) appendIfElseBlock(ifElseBlock *[]ast.Statement, count int) {
	for i := 0; i < count; i++ {
		*ifElseBlock = append(*ifElseBlock, &ast.IfStatement{
			Expression: c.convStrExpToExpStatement(<-c.falseCondition),
		})
	}
}

func (c *Obfuscator) appendConditions(exp ast.Statement) ast.Statement {
	if !c.conf.ChangeConditions {
		return exp
	}

	if c.rand.Intn(2) == 1 {
		return &ast.ExpStatement{
			Operation: ast.OpAnd,
			Left:      exp,
			Right:     c.convStrExpToExpStatement(<-c.trueCondition),
		}
	} else {
		return &ast.ExpStatement{
			Operation: ast.OpAnd,
			Left:      c.convStrExpToExpStatement(<-c.trueCondition),
			Right:     exp,
		}
	}
}

func (c *Obfuscator) expLess100() *ast.ExpStatement {
	// fname := c.appendRandFunc()

	return &ast.ExpStatement{
		Operation: 0,
		Left: &ast.ExpStatement{
			Operation: 3,
			Left: &ast.ExpStatement{
				Operation: 2,
				Left: &ast.ExpStatement{
					Operation: 0,
					Left: &ast.ExpStatement{
						Operation: 2,
						Left:      2.000000,
						Right:     float64(c.rand.Intn(14)),
					},
					Right: &ast.ExpStatement{
						Operation: 2,
						Left:      3.000000,
						Right:     float64(c.rand.Intn(14)),
					},
				},
				Right: float64(c.rand.Intn(14)),
			},
			Right: 5.000000,
		},
		Right: 7.000000,
	}
}

func (c *Obfuscator) newTernary(trueValue interface{}, depth, trueStep int) ast.TernaryStatement {

	if depth < trueStep {
		depth, trueStep = trueStep, depth
	}

	expression := c.convStrExpToExpStatement(<-c.falseCondition)
	value := c.fakeValue(trueValue)

	if trueStep == 0 {
		expression = c.convStrExpToExpStatement(<-c.trueCondition)
		value = trueValue
	}

	if depth == 0 {
		return ast.TernaryStatement{
			Expression: expression,
			TrueBlock:  value,
			ElseBlock:  c.fakeValue(trueValue),
		}
	}

	return ast.TernaryStatement{
		Expression: expression,
		TrueBlock:  value,
		ElseBlock:  c.newTernary(trueValue, depth-1, trueStep-1),
	}
}

func (c *Obfuscator) fakeValue(value interface{}) interface{} {
	switch value.(type) {
	case float64:
		return float64(c.rand.Intn(1000))
	case int:
		return float64(c.rand.Intn(1000))
	case string:
		return c.randomString(10)
	case *ast.ExpStatement:
		return c.convStrExpToExpStatement(<-c.falseCondition)
	default:
		return value
	}
}

func (c *Obfuscator) randomString(lenStr int) (result string) {
	charset := []rune("abcdefghijklmnopqrstuvwxyzйцукенгшщзхъфывапролджэячсмитьбю")
	builder := strings.Builder{}

	for builder.Len() < lenStr {
		builder.WriteString(string(charset[c.rand.Intn(len(charset))]))
	}

	return builder.String()
}

func (c *Obfuscator) obfuscateString(str string, key int32) string {
	var decrypted []rune
	for _, c := range str {
		decrypted = append(decrypted, c^key)
		// if unicode.IsLetter(c) {
		// 	decrypted = append(decrypted, c^key)
		// } else {
		// 	decrypted = append(decrypted, c)
		// }
	}

	b := []byte(string(decrypted))
	dst := make([]byte, base64.StdEncoding.EncodedLen(len(b)))
	base64.StdEncoding.Encode(dst, b)
	return string(dst)
}

func (c *Obfuscator) newDecodeStringFunc(directive string) string {
	strParam := c.randomString(10)
	keyParam := c.randomString(10)
	returnName := c.randomString(10)
	funcName := c.randomString(30)

	f := &ast.FunctionOrProcedure{
		Type: ast.PFTypeFunction,
		Name: funcName,
		Body: []ast.Statement{
			&ast.ExpStatement{
				Operation: 4,
				Left: ast.VarStatement{
					Name: strParam,
				},
				Right: ast.MethodStatement{
					Name: "ПолучитьСтрокуИзДвоичныхДанных",
					Param: []ast.Statement{
						c.hideValue(ast.MethodStatement{
							Name: "Base64Значение",
							Param: []ast.Statement{
								ast.VarStatement{
									Name: strParam,
								},
							},
						}, 4, true),
					},
				},
			},
			&ast.ExpStatement{
				Operation: ast.OpEq,
				Left: ast.VarStatement{
					Name: returnName,
				},
				Right: c.hideValue("", 4, true),
			},
			&ast.LoopStatement{
				Body: []ast.Statement{
					&ast.ExpStatement{
						Operation: ast.OpEq,
						Left: ast.VarStatement{
							Name: "код",
						},
						Right: c.hideValue(ast.MethodStatement{
							Name: "КодСимвола",
							Param: []ast.Statement{
								ast.VarStatement{
									Name: strParam,
								},
								ast.VarStatement{
									Name: "_",
								},
							},
						}, 4, true),
					},
					&ast.ExpStatement{
						Operation: ast.OpEq,
						Left: ast.VarStatement{
							Name: returnName,
						},
						Right: c.hideValue(&ast.ExpStatement{
							Operation: ast.OpPlus,
							Left: ast.VarStatement{
								Name: returnName,
							},
							Right: ast.MethodStatement{
								Name: "Символ",
								Param: []ast.Statement{
									c.hideValue(ast.MethodStatement{
										Name: "ПобитовоеИли",
										Param: []ast.Statement{
											c.hideValue(ast.MethodStatement{
												Name: "ПобитовоеИНе",
												Param: []ast.Statement{
													ast.VarStatement{
														Name: "код",
													},
													ast.VarStatement{
														Name: keyParam,
													},
												},
											}, 4, true),
											c.hideValue(ast.MethodStatement{
												Name: "ПобитовоеИНе",
												Param: []ast.Statement{
													ast.VarStatement{
														Name: keyParam,
													},
													c.hideValue(ast.VarStatement{
														Name: "код",
													}, 5, true),
												},
											}, 4, true),
										},
									}, 7, true),
								},
							},
						}, 8, true),
					},
				},
				To: ast.MethodStatement{
					Name: "СтрДлина",
					Param: []ast.Statement{
						ast.VarStatement{
							Name: strParam,
						},
					},
				},
				For: &ast.ExpStatement{
					Operation: ast.OpEq,
					Left: ast.VarStatement{
						Name: "_",
					},
					Right: 1.000000,
				},
			},
			&ast.ReturnStatement{
				Param: ast.VarStatement{
					Name: returnName,
				},
			},
		},
		Params: []ast.ParamStatement{
			{Name: strParam},
			{Name: keyParam},
		},
		Directive: directive,
	}

	c.appendGarbage(&f.Body)
	c.appendGarbage(&f.Body[2].(*ast.LoopStatement).Body)

	c.replaceLoopToGoto(&f.Body, f.Body[2].(*ast.LoopStatement), true)

	c.a.ModuleStatement.Body = append(c.a.ModuleStatement.Body, f)
	return funcName
}

func (c *Obfuscator) genCondition() {
	expresion := func(op string) (string, bool) {
		left := c.randomMathExp(c.rand.Intn(5) + 2)
		right := c.randomMathExp(c.rand.Intn(5) + 2)

		expression, err := govaluate.NewEvaluableExpression(left + op + right)
		if err != nil {
			fmt.Println(errors.Wrap(err, "genCondition error"))
			return "", false
		}

		result, _ := expression.Evaluate(nil)
		if v, ok := result.(bool); v && ok {
			return left + op + right, true
		} else if ok && !v {
			return left + op + right, false
		}

		return "", false
	}

	// true
	go func() {
		defer close(c.trueCondition)

		for {
			select {
			case <-c.ctx.Done():
				return
			default:
				if exp, ok := expresion(">"); ok {
					c.trueCondition <- exp
				}
				if exp, ok := expresion("<"); ok {
					c.trueCondition <- exp
				}
			}
		}
	}()

	// false
	go func() {
		defer close(c.falseCondition)

		for {
			select {
			case <-c.ctx.Done():
				return
			default:
				if exp, ok := expresion(">"); !ok && exp != "" {
					c.falseCondition <- exp
				}
				if exp, ok := expresion("<"); !ok && exp != "" {
					c.falseCondition <- exp
				}
			}
		}
	}()
}

func (c *Obfuscator) randomMathExp(lenExp int) (result string) {
	builder := strings.Builder{}
	defer func() { result = builder.String() }()

	operations := []string{"-", "+", "/", "*"}

	for i := 0; i < lenExp; i++ {
		builder.WriteString(strconv.Itoa(c.rand.Intn(100) + 1))
		if i < lenExp-1 {
			builder.WriteString(operations[c.rand.Intn(len(operations))])
		}
	}

	return
}

func (c *Obfuscator) convStrExpToExpStatement(str string) *ast.ExpStatement {
	astObj := ast.NewAST(fmt.Sprintf(`Процедура dsds() %s КонецПроцедуры`, str))
	if err := astObj.Parse(); err != nil {
		fmt.Println(errors.Wrap(err, "ast parse error"))
		return new(ast.ExpStatement)
	}

	return astObj.ModuleStatement.Body[0].(*ast.FunctionOrProcedure).Body[0].(*ast.ExpStatement)
}

func (c *Obfuscator) loopToGoto(loop *ast.LoopStatement) []ast.Statement {
	start := &ast.GoToLabelStatement{Name: c.randomString(5)}
	end := &ast.GoToLabelStatement{Name: c.randomString(5)}

	// цикл Пока
	if loop.WhileExpr != nil {
		newBody := []ast.Statement{
			start,
			&ast.IfStatement{
				Expression: loop.WhileExpr.(*ast.ExpStatement).Not(),
				TrueBlock:  []ast.Statement{ast.GoToStatement{Label: end}},
			},
		}

		newBody = append(append(newBody, loop.Body...), ast.GoToStatement{Label: start}, end)
		return newBody
	}

	// цикл Для а = 0 По n Цикл
	if loop.To != nil {
		exp, ok := loop.For.(*ast.ExpStatement)
		if !ok {
			return []ast.Statement{loop}
		}

		newBody := []ast.Statement{
			exp,
			start,
			&ast.IfStatement{
				Expression: &ast.ExpStatement{
					Operation: ast.OpGt,
					Left:      exp.Left,
					Right:     loop.To,
				},
				TrueBlock: []ast.Statement{ast.GoToStatement{Label: end}},
			},
		}

		newBody = append(append(newBody, loop.Body...),
			&ast.ExpStatement{
				Operation: ast.OpEq,
				Left:      exp.Left,
				Right: &ast.ExpStatement{
					Operation: ast.OpPlus,
					Left:      exp.Left,
					Right:     float64(1),
				},
			},
			ast.GoToStatement{Label: start},
			end)
		return newBody
	}

	return []ast.Statement{loop}
}

func (c *Obfuscator) replaceLoopToGoto(body *[]ast.Statement, loop *ast.LoopStatement, force bool) {
	if c.conf.RepLoopByGoto || force {
		newStatements := c.loopToGoto(loop)
		for i := len(*body) - 1; i >= 0; i-- {
			if (*body)[i] == loop {
				*body = append(append(append([]ast.Statement{}, (*body)[:i]...), newStatements...), (*body)[i+1:]...)
			}
		}
	}
}

func (c *Obfuscator) shuffleExpressions(body []ast.Statement) []ast.Statement {
	// if !c.conf.ShuffleExpressions {
	// 	return body
	// }

	orderMap := make(map[int]string, len(body))
	expr := make(map[int]ast.Statement, len(body))
	for i, item := range body {
		orderMap[i] = c.randomString(10)
		expr[i] = item
	}

	orderMap[len(body)] = c.randomString(10)

	newBody := make([]ast.Statement, 0, len(body))
	start := &ast.GoToLabelStatement{Name: orderMap[0]}
	end := &ast.GoToLabelStatement{Name: orderMap[len(body)]}
	newBody = append(newBody, ast.GoToStatement{Label: start})

	for k, v := range expr {
		next := &ast.GoToLabelStatement{Name: orderMap[k+1]}
		newBody = append(newBody, &ast.GoToLabelStatement{Name: orderMap[k]}, v, ast.GoToStatement{Label: next})
	}

	newBody = append(newBody, end)
	return newBody
}
