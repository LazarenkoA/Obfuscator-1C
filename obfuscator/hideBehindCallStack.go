package obfuscator

import (
	"github.com/LazarenkoA/1c-language-parser/ast"
	"time"
)

func (c *Obfuscator) hideBehindCallStack(directive string, val ast.ExprStatements, deep int) {
	if len(val.Statements) == 0 || len(val.Statements) > 1 {
		return
	}

	switch val.Statements[0].(type) {
	case string, int, int32, int64, float32, float64, time.Time, bool:
		funcName := c.createFakeFunc(directive, val.Statements[0])
		c.wrapFunc(directive, funcName, deep)

		val.Statements[0] = ast.MethodStatement{Name: funcName}
	}
}

func (c *Obfuscator) wrapFunc(directive string, callFuncName string, deep int) {
	if deep == 0 {
		return
	}

	funcName := c.createFakeFunc(directive, ast.MethodStatement{Name: callFuncName})
	c.wrapFunc(directive, funcName, deep-1)
}

func (c *Obfuscator) createFakeFunc(directive string, value ast.Statement) string {
	funcName := c.randomString(30)

	f := &ast.FunctionOrProcedure{
		Type:      ast.PFTypeFunction,
		Name:      funcName,
		Body:      ast.Statements{},
		Params:    []ast.ParamStatement{}, // todo можно подумать что б пробрасывать что-то из основной обфусцируемой функции
		Directive: directive,
	}

	if random(0, 2) == 1 {
		c.appendGarbage(&f.Body)
	}
	if random(0, 2) == 1 {
		c.appendGarbage(&f.Body)
	}
	if random(0, 2) == 1 {
		c.appendGarbage(&f.Body)
	}

	f.Body = append(f.Body, &ast.ReturnStatement{Param: value})

	if random(0, 2) == 1 {
		c.appendGarbage(&f.Body)
	}
	if random(0, 2) == 1 {
		c.appendGarbage(&f.Body)
	}

	c.replaceAllLoopToGoto(&f.Body)
	c.a.ModuleStatement.Body = append(c.a.ModuleStatement.Body, f)

	return funcName
}
