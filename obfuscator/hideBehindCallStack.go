package obfuscator

import (
	"time"

	"github.com/LazarenkoA/1c-language-parser/ast"
)

// hideBehindCallStack прячет константу val за цепочкой вложенных фейковых функций:
// на месте значения остается вызов функции, которая сама вызывает следующую и т.д.
// deep определяет глубину цепочки.
func (c *Obfuscator) hideBehindCallStack(directives []*ast.DirectiveStatement, val ast.ExprStatements, deep int) {
	if len(val.Statements) == 0 || len(val.Statements) > 1 {
		return
	}

	switch val.Statements[0].(type) {
	case string, int, int32, int64, float32, float64, time.Time, bool:
		funcName := c.createFakeFunc(directives, val.Statements[0])
		c.wrapFunc(directives, funcName, deep)

		val.Statements[0] = ast.MethodStatement{Name: funcName}
	}
}

// wrapFunc оборачивает вызов функции callFuncName в еще одну фейковую функцию.
// deep определяет, сколько уровней оберток создать.
func (c *Obfuscator) wrapFunc(directives []*ast.DirectiveStatement, callFuncName string, deep int) {
	if deep == 0 {
		return
	}

	funcName := c.createFakeFunc(directives, ast.MethodStatement{Name: callFuncName})
	c.wrapFunc(directives, funcName, deep-1)
}

// createFakeFunc создает функцию, возвращающую value, обрамленную случайным
// количеством мусорных выражений, и добавляет ее в тело модуля.
// Возвращает имя созданной функции.
func (c *Obfuscator) createFakeFunc(directives []*ast.DirectiveStatement, value ast.Statement) string {
	funcName := c.randomString(30)

	f := &ast.FunctionOrProcedure{
		Type:       ast.PFTypeFunction,
		Name:       funcName,
		Body:       ast.Statements{},
		Params:     []ast.ParamStatement{}, // todo можно подумать что б пробрасывать что-то из основной обфусцируемой функции
		Directives: directives,
	}

	for i := 0; i < 3; i++ {
		if random(0, 2) == 1 {
			c.appendGarbage(&f.Body)
		}
	}

	f.Body = append(f.Body, &ast.ReturnStatement{Param: value})

	for i := 0; i < 2; i++ {
		if random(0, 2) == 1 {
			c.appendGarbage(&f.Body)
		}
	}

	c.replaceAllLoopToGoto(&f.Body)
	c.a.ModuleStatement.Body = append(c.a.ModuleStatement.Body, f)

	return funcName
}
