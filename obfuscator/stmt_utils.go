package obfuscator

import "github.com/LazarenkoA/1c-language-parser/ast"

func (c *Obfuscator) isIf(stm *ast.Statement) bool {
	_, ok := (*stm).(*ast.IfStatement)
	return ok
}

func (c *Obfuscator) isExp(stm *ast.Statement) bool {
	_, ok := (*stm).(*ast.ExpStatement)
	return ok
}

func (c *Obfuscator) isMethod(stm *ast.Statement) bool {
	_, ok := (*stm).(ast.MethodStatement)
	return ok
}

func (c *Obfuscator) isFP(stm *ast.Statement) bool {
	_, ok := (*stm).(*ast.FunctionOrProcedure)
	return ok
}

func (c *Obfuscator) isLoop(stm ast.Statement) bool {
	_, ok1 := stm.(*ast.LoopStatement)
	_, ok2 := stm.(ast.LoopStatement)
	return ok1 || ok2
}
