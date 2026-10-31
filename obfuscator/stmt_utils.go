package obfuscator

import "github.com/LazarenkoA/1c-language-parser/ast"

// isExp возвращает true, если выражение является ast.ExpStatement.
func (c *Obfuscator) isExp(stm *ast.Statement) bool {
	_, ok := (*stm).(*ast.ExpStatement)
	return ok
}

// isMethod возвращает true, если выражение является вызовом метода (ast.MethodStatement).
func (c *Obfuscator) isMethod(stm *ast.Statement) bool {
	_, ok := (*stm).(ast.MethodStatement)
	return ok
}

// isFP возвращает true, если выражение является функцией или процедурой.
func (c *Obfuscator) isFP(stm *ast.Statement) bool {
	_, ok := (*stm).(*ast.FunctionOrProcedure)
	return ok
}
