package obfuscator

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log/slog"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/LazarenkoA/1c-language-parser/ast"
	"github.com/knetic/govaluate"
	"github.com/pkg/errors"
	"github.com/samber/lo"
)

// Config определяет набор преобразований, которые будут применены к коду.
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

	// CallStackHell прятать выражения за большим количеством фейковых функций
	CallStackHell bool

	// LineBreaks false - результат будет в одну строку
	LineBreaks bool
}

// Obfuscator выполняет обфускацию BSL-кода.
// Экземпляр не потокобезопасен и рассчитан на многократный вызов Obfuscate
// из одной горутины (каждый вызов обрабатывает код независимо).
type Obfuscator struct {
	ctx  context.Context
	conf Config

	// a - AST исходного (обфусцируемого) модуля
	a *ast.AstNode

	// decodeStringFuncName кэш имен функций расшифровки строк.
	// Ключ - директивы функции, значение - сгенерированное имя функции.
	decodeStringFuncName map[string]string
}

// NewObfuscator создает новый экземпляр обфускатора с указанной конфигурацией.
// ctx используется для отмены обфускации.
func NewObfuscator(ctx context.Context, conf Config) *Obfuscator {
	return &Obfuscator{
		ctx:                  ctx,
		conf:                 conf,
		decodeStringFuncName: make(map[string]string),
	}
}

// NewObfuscatory устаревший псевдоним NewObfuscator, оставлен для совместимости.
//
// Deprecated: используйте NewObfuscator.
func NewObfuscatory(ctx context.Context, conf Config) *Obfuscator {
	return NewObfuscator(ctx, conf)
}

// Obfuscate обфусцирует BSL-код и возвращает результат.
// Если код не содержит процедур и функций, он возвращается без изменений.
func (c *Obfuscator) Obfuscate(code string) (string, error) {
	if err := c.ctx.Err(); err != nil {
		return "", errors.Wrap(err, "obfuscate canceled")
	}

	c.a = ast.NewAST(code)
	if err := c.a.Parse(); err != nil {
		return "", err
	}

	if len(c.a.ModuleStatement.Body) == 0 {
		return code, nil
	}

	// сбрасываем кэш: имена функций расшифровки действительны только
	// в пределах текущего AST (функции добавляются в его тело)
	c.decodeStringFuncName = make(map[string]string)

	c.a.ModuleStatement.Walk(func(root *ast.FunctionOrProcedure, parentStm, stm *ast.Statement) {
		c.walkStep(root, parentStm, stm)
	})

	result := c.a.Print(ast.PrintConf{OneLine: !c.conf.LineBreaks, Margin: 1})
	// result = strings.ToLower(result) // нельзя так делать, все поломает
	return result, nil
}

// walkStep обрабатывает один узел AST: применяет к нему включенные в Config
// преобразования (сокрытие строк, тернарные операторы, Вычислить/Выполнить и т.д.).
func (c *Obfuscator) walkStep(currentFP *ast.FunctionOrProcedure, parent, item *ast.Statement) {
	if currentFP == nil {
		slog.Warn("you can obfuscate a procedure or function only")
		return
	}

	key := float64(random(10, 100))

	switch v := (*item).(type) {
	case string:
		if c.conf.HideString {
			*item = c.createObfuscateStringStatement(currentFP.Directives, v, int32(key))
		}
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
				v.Param.Statements[i] = casted
			case string:
				if c.conf.HideString {
					v.Param.Statements[i] = c.createObfuscateStringStatement(currentFP.Directives, casted, int32(key))
				}
			case ast.VarStatement:
				if c.conf.RepExpByTernary {
					v.Param.Statements[i] = c.hideValue(v.Param.Statements[i], 3)
				}
			}
		}

		if c.conf.RepExpByEval && parent == nil {
			str := trimTrailingSemicolon(c.a.PrintStatementWithConf(v, ast.PrintConf{}))

			*item = ast.MethodStatement{
				Name: "Выполнить",
				Param: ast.ExprStatements{
					Statements: ast.Statements{c.createObfuscateStringStatement(currentFP.Directives, str, int32(key))},
				},
			}
		}
	case *ast.ReturnStatement:
		if str, ok := v.Param.(string); ok && c.conf.HideString {
			v.Param = c.createObfuscateStringStatement(currentFP.Directives, str, int32(key))
		}
	case *ast.ExpStatement:
		c.obfuscateExpStatement(currentFP, (*interface{})(item))

		if _, ok := v.Left.(ast.VarStatement); ok && c.conf.RepExpByEval {
			switch v.Right.(type) {
			case ast.MethodStatement, ast.CallChainStatement, ast.NewObjectStatement:
				str := trimTrailingSemicolon(c.a.PrintStatementWithConf(v.Right, ast.PrintConf{}))

				v.Right = ast.MethodStatement{
					Name: "Вычислить",
					Param: ast.ExprStatements{Statements: ast.Statements{ast.MethodStatement{
						Name:  c.decodeStringFunc(currentFP.Directives),
						Param: ast.ExprStatements{Statements: ast.Statements{c.obfuscateString(str, int32(key)), c.hideValue(key, 4)}},
					}}},
				}
			default:
				v.Right = c.hideValue(v.Right, 4)
			}
		}
	case ast.CallChainStatement:
		if c.conf.RepExpByEval && (c.isMethod(parent) || c.isExp(parent) || c.isFP(parent)) {
			str := trimTrailingSemicolon(c.a.PrintStatementWithConf(v, ast.PrintConf{}))

			*item = ast.MethodStatement{
				Name: ast.IF(c.isMethod(parent) || c.isExp(parent), "Вычислить", "Выполнить"),
				Param: ast.ExprStatements{Statements: ast.Statements{
					ast.MethodStatement{
						Name:  c.decodeStringFunc(currentFP.Directives),
						Param: ast.ExprStatements{Statements: ast.Statements{c.obfuscateString(str, int32(key)), c.hideValue(key, 4)}},
					},
				}},
			}
		}
	case *ast.LoopStatement:
		c.replaceLoopToGoto(&currentFP.Body, v, false)
	case ast.ExprStatements:
		for i := range v.Statements {
			c.walkStep(currentFP, item, &v.Statements[i])
		}
	case ast.ThrowStatement:
		switch casted := v.Param.(type) {
		case *ast.ExpStatement, ast.MethodStatement, ast.ExprStatements:
			c.walkStep(currentFP, item, &casted)
			v.Param = casted
		}
		*item = v
	case ast.AssignmentStatement:
		expr := ast.Statement(v.Expr)
		c.walkStep(currentFP, item, &expr)
		if casted, ok := expr.(ast.ExprStatements); ok {
			v.Expr = casted
		}

		if c.conf.CallStackHell {
			c.hideBehindCallStack(currentFP.Directives, v.Expr, int(random(3, 7)))
		}
		*item = v
	case ast.NewObjectStatement:
		param := ast.Statement(v.Param)
		c.walkStep(currentFP, item, &param)
		if casted, ok := param.(ast.ExprStatements); ok {
			v.Param = casted
		}
		*item = v
	case ast.ItemStatement:
		c.walkStep(currentFP, item, &v.Object)
		*item = v
	}
}

// obfuscateExpStatement рекурсивно обходит части выражения и применяет
// преобразования к его правой/левой частям, строкам и параметрам методов.
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
			*part = c.createObfuscateStringStatement(currentPF.Directives, r, int32(key))
		}
		return
	case ast.ReturnStatement:
		if str, ok := r.Param.(string); ok && c.conf.HideString {
			r.Param = ast.MethodStatement{
				Name:  c.decodeStringFunc(currentPF.Directives),
				Param: ast.ExprStatements{Statements: ast.Statements{c.obfuscateString(str, int32(key)), c.hideValue(key, 4)}},
			}
		}
	case ast.IParams:
		for i, param := range r.Params() {
			if str, ok := param.(string); ok && c.conf.HideString {
				r.Params()[i] = c.createObfuscateStringStatement(currentPF.Directives, str, int32(key))
			}
		}
	}
}

// createObfuscateStringStatement строит вызов функции расшифровки строк,
// в которую зашита обфусцированная строка и ключ расшифровки.
func (c *Obfuscator) createObfuscateStringStatement(directive []*ast.DirectiveStatement, str string, key int32) ast.MethodStatement {
	return ast.MethodStatement{
		Name:  c.decodeStringFunc(directive),
		Param: ast.ExprStatements{Statements: ast.Statements{c.obfuscateString(str, key), c.hideValue(key, 4)}},
	}
}

// decodeStringFunc возвращает имя функции расшифровки строк для указанных директив.
// Для каждого набора директив функция создается один раз и кэшируется.
func (c *Obfuscator) decodeStringFunc(directives []*ast.DirectiveStatement) string {
	key := strings.Join(directiveNames(directives), "")
	if name, ok := c.decodeStringFuncName[key]; ok {
		return name
	}

	name := c.createDecodeStringFunc(directives)
	c.decodeStringFuncName[key] = name

	return name
}

// directiveNames возвращает имена переданных директив.
func directiveNames(directives []*ast.DirectiveStatement) []string {
	return lo.Map(directives, func(d *ast.DirectiveStatement, _ int) string {
		return d.Name
	})
}

// hideValue прячет значение за цепочкой вложенных тернарных операторов.
// Значения неподдерживаемых типов возвращаются как есть.
func (c *Obfuscator) hideValue(val interface{}, complexity int) ast.Statement {
	switch val.(type) {
	case string, bool, float64, int, int32, int64, float32, time.Time, *ast.ExpStatement, ast.MethodStatement, ast.VarStatement:
		return c.newTernary(val, int(random(2, complexity)), int(random(0, complexity-1)))
	default:
		return val
	}
}

// appendGarbage добавляет в тело мусорные выражения, условия и циклы,
// которые никогда не выполняются. Работает только если включен Config.AppendGarbage.
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
		IF := &ast.IfStatement{Expression: c.convStrExpToExpStatement(c.genCondition(false))}

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
		loop := &ast.LoopStatement{WhileExpr: c.convStrExpToExpStatement(c.genCondition(false))}
		if random(0, 2) == 1 {
			c.appendGarbage(&loop.Body)
		}

		loop.Body = c.shuffleExpressions(loop.Body)
		*body = append(*body, loop)
	}
}

// appendIfElseBlock добавляет count веток ИначеЕсли с заведомо ложными условиями.
func (c *Obfuscator) appendIfElseBlock(ifElseBlock *ast.Statements, count int) {
	for i := 0; i < count; i++ {
		*ifElseBlock = append(*ifElseBlock, &ast.IfStatement{
			Expression: c.convStrExpToExpStatement(c.genCondition(false)),
		})
	}
}

// appendConditions оборачивает условие в цепочку заведомо истинных условий.
// Работает только если включен Config.ChangeConditions.
func (c *Obfuscator) appendConditions(exp ast.Statement) ast.Statement {
	if !c.conf.ChangeConditions {
		return exp
	}

	return c.helperAppendConditions(exp, 3)
}

// helperAppendConditions рекурсивно добавляет к условию depth истинных операндов через И.
func (c *Obfuscator) helperAppendConditions(exp ast.Statement, depth int) ast.Statement {
	if depth == 0 {
		return exp
	}

	newConditions := &ast.ExpStatement{
		Operation: ast.OpAnd,
		Left:      exp,
		Right:     c.convStrExpToExpStatement(c.genCondition(true)),
	}

	if random(0, 2) == 1 {
		newConditions = &ast.ExpStatement{
			Operation: ast.OpAnd,
			Left:      c.convStrExpToExpStatement(c.genCondition(true)),
			Right:     exp,
		}
	}

	return c.helperAppendConditions(newConditions, depth-1)
}

// newTernary строит цепочку вложенных тернарных операторов, на глубине trueStep
// которой возвращается trueValue, на остальных уровнях - фиктивное значение.
func (c *Obfuscator) newTernary(trueValue interface{}, depth, trueStep int) ast.TernaryStatement {
	if depth < trueStep {
		depth, trueStep = trueStep, depth
	}

	expression := c.convStrExpToExpStatement(c.genCondition(false))
	value := c.fakeValue(trueValue)

	if trueStep == 0 {
		expression = c.convStrExpToExpStatement(c.genCondition(true))
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

// fakeValue возвращает фиктивное значение того же типа, что и value.
// Для неподдерживаемых типов возвращается само значение.
func (c *Obfuscator) fakeValue(value interface{}) interface{} {
	switch value.(type) {
	case float64, float32, int, int32, int64:
		return float64(random(0, 1000))
	case string:
		return c.randomString(10)
	case *ast.ExpStatement:
		return c.convStrExpToExpStatement(c.genCondition(false))
	case ast.MethodStatement:
		return c.fakeMethods()
	default:
		return value
	}
}

// fakeMethods возвращает вызов случайного платформенного метода со случайными параметрами.
// Важно, чтобы методы были доступны и на клиенте, и на сервере.
func (c *Obfuscator) fakeMethods() ast.MethodStatement {
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

// randomString генерирует случайную строку из строчных латинских букв заданной длины.
func (c *Obfuscator) randomString(lenStr int) string {
	charset := []rune("abcdefghijklmnopqrstuvwxyz")
	builder := strings.Builder{}
	builder.Grow(lenStr)

	for i := 0; i < lenStr; i++ {
		builder.WriteRune(charset[random(0, len(charset))])
	}

	return builder.String()
}

// obfuscateString шифрует строку XOR-ом с ключом key и кодирует результат в base64.
// Переносы строк в начале строки (символ |) заменяются на пробел,
// т.к. 1С интерпретирует | первым символом строки как перенос строки.
func (c *Obfuscator) obfuscateString(str string, key int32) string {
	lines := strings.Split(str, "\n")
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "|") {
			lines[i] = line[:len(line)-len(trimmed)] + " " + trimmed[1:]
		}
	}
	str = strings.Join(lines, "\n")
	str = strings.ReplaceAll(str, `""`, `"`) // заменяем двойные кавычки на одинарные

	encrypted := make([]rune, 0, len(str))
	for _, ch := range str {
		encrypted = append(encrypted, ch^key)
	}

	b := []byte(string(encrypted))
	dst := make([]byte, base64.StdEncoding.EncodedLen(len(b)))
	base64.StdEncoding.Encode(dst, b)

	return string(dst)
}

// createDecodeStringFunc генерирует функцию расшифровки строк (XOR + base64)
// и добавляет ее в тело модуля. Возвращает имя созданной функции.
func (c *Obfuscator) createDecodeStringFunc(directives []*ast.DirectiveStatement) string {
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
		Directives: lo.Filter(directives, func(item *ast.DirectiveStatement, index int) bool {
			return item.Src == ""
		}),
	}

	c.appendGarbage(&f.Body)

	// декодирующий цикл - третий по счету элемент тела функции
	loop, ok := f.Body[2].(*ast.LoopStatement)
	if !ok {
		slog.Error("decode string func: loop statement expected at body index 2")
		return ""
	}

	c.appendGarbage(&loop.Body)
	c.replaceLoopToGoto(&f.Body, loop, true)

	c.a.ModuleStatement.Body = append(c.a.ModuleStatement.Body, f)

	return funcName
}

// genCondition генерирует случайное математическое выражение,
// которое гарантированно истинно (needTrue == true) или ложно (needTrue == false).
func (c *Obfuscator) genCondition(needTrue bool) string {
	for {
		for _, op := range []string{">", "<"} {
			left := c.randomMathExp(int(random(2, 7)))
			right := c.randomMathExp(int(random(2, 7)))
			expStr := left + op + right

			expression, err := govaluate.NewEvaluableExpression(expStr)
			if err != nil {
				slog.Error("genCondition error", "err", err)
				continue
			}

			result, err := expression.Evaluate(nil)
			if err != nil {
				slog.Error("genCondition evaluate error", "err", err)
				continue
			}

			if v, ok := result.(bool); ok && v == needTrue {
				return expStr
			}
		}
	}
}

// randomMathExp генерирует случайное математическое выражение из lenExp чисел,
// соединенных случайными арифметическими операциями.
func (c *Obfuscator) randomMathExp(lenExp int) string {
	operations := []string{"-", "+", "/", "*"}

	builder := strings.Builder{}
	for i := 0; i < lenExp; i++ {
		builder.WriteString(strconv.Itoa(int(random(1, 1000))))
		if i < lenExp-1 {
			builder.WriteString(operations[random(0, len(operations))])
		}
	}

	return builder.String()
}

// convStrExpToExpStatement парсит строку с выражением в ast.ExpStatement.
// В случае ошибки парсинга возвращает пустое выражение.
func (c *Obfuscator) convStrExpToExpStatement(str string) *ast.ExpStatement {
	astObj := ast.NewAST(str)
	if err := astObj.Parse(); err != nil {
		slog.Error("ast parse error", "err", err)
		return new(ast.ExpStatement)
	}

	return astObj.ModuleStatement.Body[0].(*ast.ExpStatement)
}

// loopToGoto преобразует цикл в эквивалентный набор операторов Перейти.
// Прервать заменяется на переход в конец цикла, Продолжить - в начало.
// Если цикл преобразовать нельзя, он возвращается без изменений.
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

		newBody = append(newBody, loop.Body...)
		newBody = append(newBody, ast.GoToStatement{Label: start}, end)
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

		newBody = append(newBody, loop.Body...)
		newBody = append(newBody,
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

// invertExp инвертирует выражение. Для типов без поддержки инверсии
// выражение возвращается как есть.
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

// replaceLoopToGoto заменяет цикл loop в теле body на набор операторов Перейти.
// force позволяет выполнить замену независимо от Config.RepLoopByGoto.
func (c *Obfuscator) replaceLoopToGoto(body *ast.Statements, loop *ast.LoopStatement, force bool) {
	if !c.conf.RepLoopByGoto && !force {
		return
	}

	newStatements := c.loopToGoto(loop)
	for i := len(*body) - 1; i >= 0; i-- {
		if (*body)[i] == loop {
			*body = append(append(append(ast.Statements{}, (*body)[:i]...), newStatements...), (*body)[i+1:]...)
		}
	}
}

// replaceAllLoopToGoto заменяет все циклы в теле body на наборы операторов Перейти.
func (c *Obfuscator) replaceAllLoopToGoto(body *ast.Statements) {
	for i := len(*body) - 1; i >= 0; i-- {
		var newStatements ast.Statements

		switch v := (*body)[i].(type) {
		case *ast.LoopStatement:
			newStatements = c.loopToGoto(v)
		case ast.LoopStatement:
			newStatements = c.loopToGoto(&v)
		}

		if newStatements != nil {
			*body = append(append(append(ast.Statements{}, (*body)[:i]...), newStatements...), (*body)[i+1:]...)
		}
	}
}

// shuffleExpressions перемешивает порядок обхода выражений в теле: каждое выражение
// оборачивается в пару метка/переход, но итоговый порядок выполнения сохраняется.
func (c *Obfuscator) shuffleExpressions(body ast.Statements) ast.Statements {
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

// trimTrailingSemicolon убирает завершающую точку с запятой, т.к. выражение
// будет встроено внутрь другого выражения.
func trimTrailingSemicolon(str string) string {
	return strings.TrimSuffix(str, ";")
}

// random возвращает случайное число из диапазона [min, max).
// Для пустого диапазона (max <= min) возвращает min.
func random(min, max int) int64 {
	if max <= min {
		return int64(min)
	}

	n, err := rand.Int(rand.Reader, big.NewInt(int64(max-min)))
	if err != nil {
		slog.Error("rand error", "err", err)
		return int64(min)
	}

	return n.Int64() + int64(min)
}
