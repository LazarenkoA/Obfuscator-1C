package obfuscator

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"
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

	// RepExpByEval прятать выражения в Выполнить() Вычислить()
	RepExpByEval bool

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

	c.a.ModuleStatement.Walk(func(root *ast.FunctionOrProcedure, parentStm, stm *ast.Statement) {
		c.walkStep(root, parentStm, stm)
	})

	result := c.a.Print(ast.PrintConf{OneLine: true, Margin: 1})
	// result = strings.ToLower(result) // нельзя так делать, все поломает
	return result, nil
}

func (c *Obfuscator) walkStep(currentFP *ast.FunctionOrProcedure, parent, item *ast.Statement) {
	if currentFP == nil {
		fmt.Println("! you can obfuscate a procedure or function")
		return
	}

	key := float64(random(10, 100))

	switch v := (*item).(type) {
	case *ast.IfStatement:
		c.walkStep(currentFP, item, &v.Expression)

		v.Expression = c.appendConditions(v.Expression)
		if c.conf.ChangeConditions {
			c.appendIfElseBlock(&v.IfElseBlock, int(random(0, 5)))
			c.appendGarbage(&v.ElseBlock)
			c.appendGarbage(&v.TrueBlock)
		}

		// v.TrueBlock = c.shuffleExpressions(v.TrueBlock)
		// v.ElseBlock = c.shuffleExpressions(v.ElseBlock)
	case *ast.FunctionOrProcedure:
		c.appendGarbage(&v.Body)
		// v.Body = c.shuffleExpressions(v.Body)
	case ast.MethodStatement:
		for i, param := range v.Param.Statements {
			switch casted := param.(type) {
			case *ast.ExpStatement, ast.MethodStatement:
				c.walkStep(currentFP, item, &casted)
			case string:
				v.Param.Statements[i] = ast.MethodStatement{
					Name:  c.decodeStringFunc(currentFP.Directive),
					Param: ast.ExprStatements{Statements: ast.Statements{c.obfuscateString(casted, int32(key)), c.hideValue(key, 4)}},
				}
			}
		}

		if c.conf.RepExpByEval && parent == nil && random(0, 2) == 1 {
			str := c.a.PrintStatementWithConf(v, ast.PrintConf{})
			if str[len(str)-1] == ';' {
				str = str[:len(str)-1]
			}

			*item = ast.MethodStatement{
				Name: "Выполнить",
				Param: ast.ExprStatements{
					Statements: ast.Statements{ast.MethodStatement{
						Name:  c.decodeStringFunc(currentFP.Directive),
						Param: ast.ExprStatements{Statements: ast.Statements{c.obfuscateString(str, int32(key)), c.hideValue(key, 4)}},
					}},
				},
			}
		}
	case *ast.ReturnStatement:
		if str, ok := v.Param.(string); ok && c.conf.HideString {
			v.Param = ast.MethodStatement{
				Name:  c.decodeStringFunc(currentFP.Directive),
				Param: ast.ExprStatements{Statements: ast.Statements{c.obfuscateString(str, int32(key)), c.hideValue(key, 4)}},
			}
		}
	case *ast.ExpStatement:
		c.obfuscateExpStatement(currentFP, (*interface{})(item))

		if _, ok := v.Left.(ast.VarStatement); ok && c.conf.RepExpByEval {
			switch v.Right.(type) {
			case ast.MethodStatement, ast.CallChainStatement, ast.NewObjectStatement:
				str := c.a.PrintStatementWithConf(v.Right, ast.PrintConf{})
				if str[len(str)-1] == ';' {
					str = str[:len(str)-1]
				}

				v.Right = ast.MethodStatement{
					Name: "Вычислить",
					Param: ast.ExprStatements{Statements: ast.Statements{ast.MethodStatement{
						Name:  c.decodeStringFunc(currentFP.Directive),
						Param: ast.ExprStatements{Statements: ast.Statements{c.obfuscateString(str, int32(key)), c.hideValue(key, 4)}},
					}}},
				}
			default:
				v.Right = c.hideValue(v.Right, 4)
			}
		}
	case ast.CallChainStatement:
		if c.conf.RepExpByEval && (c.isMethod(parent) || c.isExp(parent) || c.isFP(parent)) {
			str := c.a.PrintStatementWithConf(v, ast.PrintConf{})
			if str[len(str)-1] == ';' {
				str = str[:len(str)-1]
			}

			*item = ast.MethodStatement{
				Name: ast.IF(c.isMethod(parent) || c.isExp(parent), "Вычислить", "Выполнить"),
				Param: ast.ExprStatements{Statements: ast.Statements{
					ast.MethodStatement{
						Name:  c.decodeStringFunc(currentFP.Directive),
						Param: ast.ExprStatements{Statements: ast.Statements{c.obfuscateString(str, int32(key)), c.hideValue(key, 4)}},
					},
				}},
			}
		}
	case *ast.LoopStatement:
		c.replaceLoopToGoto(&currentFP.Body, v, false)
	case ast.ExprStatements:
		for _, p := range v.Statements {
			c.walkStep(currentFP, item, &p)
		}
	case ast.ThrowStatement:
		switch casted := v.Param.(type) {
		case *ast.ExpStatement, ast.MethodStatement, ast.ExprStatements:
			c.walkStep(currentFP, item, &casted)
		}
	}
}

func (c *Obfuscator) obfuscateExpStatement(currentPF *ast.FunctionOrProcedure, part *interface{}) {
	key := float64(random(10, 100))

	switch r := (*part).(type) {
	case *ast.ExpStatement:
		c.obfuscateExpStatement(currentPF, &r.Right)
		c.obfuscateExpStatement(currentPF, &r.Left)

		if c.conf.RepExpByTernary {
			r.Right = c.hideValue(r.Right, 4)
		}
	case string:
		if c.conf.HideString {
			*part = ast.MethodStatement{
				Name:  c.decodeStringFunc(currentPF.Directive),
				Param: ast.ExprStatements{Statements: ast.Statements{c.obfuscateString(r, int32(key)), c.hideValue(key, 4)}},
			}
		}
		return
	case ast.ReturnStatement:
		if str, ok := r.Param.(string); ok && c.conf.HideString {
			r.Param = ast.MethodStatement{
				Name:  c.decodeStringFunc(currentPF.Directive),
				Param: ast.ExprStatements{Statements: ast.Statements{c.obfuscateString(str, int32(key)), c.hideValue(key, 4)}},
			}
		}
	case ast.IParams:
		for i, param := range r.Params() {
			if str, ok := param.(string); ok && c.conf.HideString {
				r.Params()[i] = ast.MethodStatement{
					Name:  c.decodeStringFunc(currentPF.Directive),
					Param: ast.ExprStatements{Statements: ast.Statements{c.obfuscateString(str, int32(key)), c.hideValue(key, 4)}},
				}
			}
		}
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

func (c *Obfuscator) hideValue(val interface{}, complexity int) ast.Statement {
	switch val.(type) {
	case string, bool, float64, int, time.Time, *ast.ExpStatement, ast.MethodStatement:
		return c.newTernary(val, int(random(2, complexity)), int(random(0, complexity-1)))
	default:
		return val
	}
}

func (c *Obfuscator) appendGarbage(body *ast.Statements) {
	if !c.conf.AppendGarbage {
		return
	}

	if random(0, 2) == 1 {
		*body = append(*body, &ast.ExpStatement{
			Operation: ast.OpEq,
			Left:      ast.VarStatement{Name: c.randomString(20)},
			Right:     c.hideValue(c.randomString(5), 4),
		})
	}
	if random(0, 2) == 1 {
		*body = append(*body, &ast.ExpStatement{
			Operation: ast.OpEq,
			Left:      ast.VarStatement{Name: c.randomString(10)},
			Right:     c.hideValue(float64(random(-100, 100)), 5),
		})
	}
	if random(0, 2) == 1 {
		IF := &ast.IfStatement{Expression: c.convStrExpToExpStatement(<-c.falseCondition)}

		if random(0, 2) == 1 {
			c.appendIfElseBlock(&IF.IfElseBlock, int(random(0, 5)))
		}
		if random(0, 2) == 1 {
			c.appendGarbage(&IF.ElseBlock)
			c.appendGarbage(&IF.TrueBlock)
		}

		IF.TrueBlock = c.shuffleExpressions(IF.TrueBlock)
		IF.ElseBlock = c.shuffleExpressions(IF.ElseBlock)
		*body = append(*body, IF)
	}
	if random(0, 2) == 1 {
		loop := &ast.LoopStatement{WhileExpr: c.convStrExpToExpStatement(<-c.falseCondition)}
		if random(0, 2) == 1 {
			c.appendGarbage(&loop.Body)
		}

		loop.Body = c.shuffleExpressions(loop.Body)
		*body = append(*body, loop)
	}
}

func (c *Obfuscator) appendIfElseBlock(ifElseBlock *ast.Statements, count int) {
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

	return c.helperAppendConditions(exp, 3)
}

func (c *Obfuscator) helperAppendConditions(exp ast.Statement, depth int) ast.Statement {
	if depth == 0 {
		return exp
	}

	newConditions := &ast.ExpStatement{
		Operation: ast.OpAnd,
		Left:      exp,
		Right:     c.convStrExpToExpStatement(<-c.trueCondition),
	}

	if random(0, 2) == 1 {
		newConditions = &ast.ExpStatement{
			Operation: ast.OpAnd,
			Left:      c.convStrExpToExpStatement(<-c.trueCondition),
			Right:     exp,
		}
	}

	return c.helperAppendConditions(newConditions, depth-1)
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
		return float64(random(0, 1000))
	case int:
		return float64(random(0, 1000))
	case string:
		return c.randomString(10)
	case *ast.ExpStatement:
		return c.convStrExpToExpStatement(<-c.falseCondition)
	case ast.MethodStatement:
		return c.fakeMethods()
	default:
		return value
	}
}

func (c *Obfuscator) fakeMethods() ast.MethodStatement {
	// массив платформенных методов (важно что б они были доступны на клиенте и на сервере)
	pool := []ast.MethodStatement{
		{
			Name:  "XMLСтрока",
			Param: ast.ExprStatements{Statements: ast.Statements{float64(random(0, 1000))}},
		},
		{
			Name:  "Лев",
			Param: ast.ExprStatements{Statements: ast.Statements{c.randomString(20), float64(random(1, 10))}},
		},
		{
			Name:  "Прав",
			Param: ast.ExprStatements{Statements: ast.Statements{c.randomString(20), float64(random(1, 10))}},
		},
		{
			Name:  "Сред",
			Param: ast.ExprStatements{Statements: ast.Statements{c.randomString(20), float64(random(1, 10)), float64(random(0, 10))}},
		},
		{
			Name:  "ПобитовыйСдвигВлево",
			Param: ast.ExprStatements{Statements: ast.Statements{float64(random(0, 1000)), float64(random(1, 10))}},
		},
		{
			Name:  "ПобитовыйСдвигВправо",
			Param: ast.ExprStatements{Statements: ast.Statements{float64(random(0, 1000)), float64(random(1, 10))}},
		},
		{
			Name:  "ПобитовоеИ",
			Param: ast.ExprStatements{Statements: ast.Statements{float64(random(0, 1000)), float64(random(1, 10))}},
		},
	}

	return pool[random(0, len(pool))]
}

func (c *Obfuscator) randomString(lenStr int) (result string) {
	charset := []rune("abcdefghijklmnopqrstuvwxyz")
	builder := strings.Builder{}

	for builder.Len() < lenStr {
		builder.WriteString(string(charset[random(0, len(charset))]))
	}

	return builder.String()
}

func (c *Obfuscator) obfuscateString(str string, key int32) string {
	var decrypted []rune
	for _, c := range strings.ReplaceAll(str, "|", " ") {
		decrypted = append(decrypted, c^key)
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
		Body: ast.Statements{
			&ast.ExpStatement{
				Operation: ast.OpEq,
				Left: ast.VarStatement{
					Name: strParam,
				},
				Right: ast.MethodStatement{
					Name: "ПолучитьСтрокуИзДвоичныхДанных",
					Param: ast.ExprStatements{Statements: ast.Statements{
						c.hideValue(ast.MethodStatement{
							Name: "Base64Значение",
							Param: ast.ExprStatements{Statements: ast.Statements{
								ast.VarStatement{
									Name: strParam,
								},
							}},
						}, 4),
					}},
				},
			},
			&ast.ExpStatement{
				Operation: ast.OpEq,
				Left: ast.VarStatement{
					Name: returnName,
				},
				Right: c.hideValue("", 4),
			},
			&ast.LoopStatement{
				Body: ast.Statements{
					&ast.ExpStatement{
						Operation: ast.OpEq,
						Left: ast.VarStatement{
							Name: "код",
						},
						Right: c.hideValue(ast.MethodStatement{
							Name: "КодСимвола",
							Param: ast.ExprStatements{Statements: ast.Statements{
								ast.VarStatement{
									Name: strParam,
								},
								ast.VarStatement{
									Name: "_",
								},
							}},
						}, 4),
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
								Param: ast.ExprStatements{Statements: ast.Statements{
									c.hideValue(ast.MethodStatement{
										Name: "ПобитовоеИли",
										Param: ast.ExprStatements{Statements: ast.Statements{
											c.hideValue(ast.MethodStatement{
												Name: "ПобитовоеИНе",
												Param: ast.ExprStatements{Statements: ast.Statements{
													ast.VarStatement{
														Name: "код",
													},
													ast.VarStatement{
														Name: keyParam,
													},
												}},
											}, 4),
											c.hideValue(ast.MethodStatement{
												Name: "ПобитовоеИНе",
												Param: ast.ExprStatements{Statements: ast.Statements{
													ast.VarStatement{
														Name: keyParam,
													},
													c.hideValue(ast.VarStatement{
														Name: "код",
													}, 5),
												}},
											}, 4),
										}},
									}, 7),
								}},
							},
						}, 8),
					},
				},
				To: ast.MethodStatement{
					Name: "СтрДлина",
					Param: ast.ExprStatements{Statements: ast.Statements{
						ast.VarStatement{
							Name: strParam,
						},
					}},
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
	expression := func(op string) (string, bool) {
		left := c.randomMathExp(int(random(2, 7)))
		right := c.randomMathExp(int(random(2, 7)))

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
				if exp, ok := expression(">"); ok {
					c.trueCondition <- exp
				}
				if exp, ok := expression("<"); ok {
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
				if exp, ok := expression(">"); !ok && exp != "" {
					c.falseCondition <- exp
				}
				if exp, ok := expression("<"); !ok && exp != "" {
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
		builder.WriteString(strconv.Itoa(int(random(1, 1000))))
		if i < lenExp-1 {
			builder.WriteString(operations[random(0, len(operations))])
		}
	}

	return
}

func (c *Obfuscator) convStrExpToExpStatement(str string) *ast.ExpStatement {
	astObj := ast.NewAST(str)
	if err := astObj.Parse(); err != nil {
		fmt.Println(errors.Wrap(err, "ast parse error"))
		return new(ast.ExpStatement)
	}

	return astObj.ModuleStatement.Body[0].(*ast.ExpStatement)
}

func (c *Obfuscator) loopToGoto(loop *ast.LoopStatement) ast.Statements {
	start := &ast.GoToLabelStatement{Name: c.randomString(5)}
	end := &ast.GoToLabelStatement{Name: c.randomString(5)}

	// цикл Пока
	if loop.WhileExpr != nil {
		newBody := ast.Statements{
			start,
			&ast.IfStatement{
				Expression: c.invertExp(loop.WhileExpr),
				TrueBlock:  ast.Statements{ast.GoToStatement{Label: end}},
			},
		}

		// меняем прервать и продолжить
		ast.StatementWalk(loop, loop.Body, func(root *ast.FunctionOrProcedure, parentStm, stm *ast.Statement) {
			switch (*stm).(type) {
			case ast.ContinueStatement:
				*stm = ast.GoToStatement{Label: start}
			case ast.BreakStatement:
				*stm = ast.GoToStatement{Label: end}
			}
		})

		newBody = append(append(newBody, loop.Body...), ast.GoToStatement{Label: start}, end)
		return newBody
	}

	// цикл Для а = 0 По n Цикл
	if loop.To != nil {
		exp, ok := loop.For.(*ast.ExpStatement)
		if !ok {
			return ast.Statements{loop}
		}

		newBody := ast.Statements{
			exp,
			start,
			&ast.IfStatement{
				Expression: &ast.ExpStatement{
					Operation: ast.OpGt,
					Left:      exp.Left,
					Right:     loop.To,
				},
				TrueBlock: ast.Statements{ast.GoToStatement{Label: end}},
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

	return ast.Statements{loop}
}

func (c *Obfuscator) invertExp(exp ast.Statement) ast.Statement {
	switch v := exp.(type) {
	case ast.INot:
		return v.Not()
	case bool:
		return !v
	default:
		return exp
	}
}

func (c *Obfuscator) replaceLoopToGoto(body *ast.Statements, loop *ast.LoopStatement, force bool) {
	if c.conf.RepLoopByGoto || force {
		newStatements := c.loopToGoto(loop)
		for i := len(*body) - 1; i >= 0; i-- {
			if (*body)[i] == loop {
				*body = append(append(append(ast.Statements{}, (*body)[:i]...), newStatements...), (*body)[i+1:]...)
			}
		}
	}
}

func (c *Obfuscator) shuffleExpressions(body ast.Statements) []ast.Statement {
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

// [min, max)
func random(min, max int) int64 {
	max -= min
	if max <= 0 {
		return 0
	}

	randomNumber, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		fmt.Println(errors.Wrap(err, "rand error"))
		return 0
	}

	return randomNumber.Int64() + int64(min)
}
