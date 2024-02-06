package obfuscator

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/LazarenkoA/1c-language-parser/ast"
	"github.com/google/uuid"
)

type Config struct {
}

type Cloak struct {
	conf *Config
	// moduleBody        []ast.Statement
	a                    *ast.AstNode
	rand                 *rand.Rand
	oneDecodeStringFunc  sync.Once
	decodeStringFuncName string
	decodeKey            int
}

func init() {

}

func NewCloak(conf *Config) *Cloak {
	return &Cloak{
		conf: conf,
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (c *Cloak) Obfuscate(code string) (string, error) {
	c.a = ast.NewAST(code)
	if err := c.a.Parse(); err != nil {
		return "", err
	}

	if len(c.a.ModuleStatement.Body) == 0 {
		return code, nil
	}

	// pp.Println(c.a.ModuleStatement)

	c.a.ModuleStatement.Walk(c.walkStep)

	// c.moduleBody = a.ModuleStatement.Body
	// for i := 0; i < c.rand.Intn(3)+1; i++ {
	// 	c.changeConditions(c.a.ModuleStatement.Body)
	// }

	str := strings.Replace(c.a.Print(&ast.PrintConf{Margin: 4}), "|", " ", -1)
	fmt.Println(str)

	return "", nil
}

func (c *Cloak) walkStep(item *ast.Statement) {
	switch v := (*item).(type) {
	case *ast.IfStatement:
		v.Expression = c.appendConditions(v.Expression)
		c.appendIfElseBlock(&v.IfElseBlock, c.rand.Intn(5))
		c.appendGarbage(&v.ElseBlock)
		c.appendGarbage(&v.TrueBlock)
	case *ast.FunctionOrProcedure:
		c.appendGarbage(&v.Body)
	case ast.MethodStatement:
		fmt.Println(1)
	case *ast.ExpStatement:
		switch r := v.Right.(type) {
		case string:
			funcName, key := c.appendDecodeString()
			v.Right = ast.MethodStatement{
				Name:  funcName,
				Param: []ast.Statement{c.obfuscateString(r, int32(key))},
			}
		case ast.NewObjectStatement:
			for i, param := range r.Param {
				if str, ok := param.(string); ok {
					funcName, key := c.appendDecodeString()
					r.Param[i] = ast.MethodStatement{
						Name:  funcName,
						Param: []ast.Statement{c.obfuscateString(str, int32(key))},
					}
				}
			}
		}

		if _, ok := v.Left.(ast.VarStatement); ok {
			v.Right = c.hideValue(v.Right, 4)
		}
	}
}

func (c *Cloak) hideValue(val interface{}, complexity int) ast.Statement {
	switch val.(type) {
	case string, bool, float64, int, time.Time, *ast.ExpStatement:
		return c.newTernary(val, c.rand.Intn(complexity-2)+2, c.rand.Intn(complexity-1))
	default:
		return val
	}
	// return c.newTernary(val, c.rand.Intn(complexity-2)+2, c.rand.Intn(complexity))
}

func (c *Cloak) appendGarbage(body *[]ast.Statement) {
	*body = append(*body, &ast.ExpStatement{
		Operation: ast.OpEq,
		Left:      ast.VarStatement{Name: c.randomString(20)},
		Right:     c.newTernary("2222", c.rand.Intn(4)+1, c.rand.Intn(4)),
	})
}

func (c *Cloak) appendIfElseBlock(ifElseBlock *[]ast.Statement, count int) {
	for i := 0; i < count; i++ {
		*ifElseBlock = append(*ifElseBlock, &ast.IfStatement{
			Expression: c.getFalseCondition(),
		})
	}
}

func (c *Cloak) getFalseCondition() *ast.ExpStatement {
	falseConditions := []func() *ast.ExpStatement{
		c.falseCondition1,
		c.falseCondition2,
		c.falseCondition3,
		c.falseCondition4,
	}

	return falseConditions[c.rand.Intn(len(falseConditions))]()
}

func (c *Cloak) getTrueCondition() *ast.ExpStatement {
	trueConditions := []func() *ast.ExpStatement{
		c.trueCondition1,
		c.trueCondition2,
		c.trueCondition3,
		c.trueCondition4,
		c.trueCondition5,
	}

	return trueConditions[c.rand.Intn(len(trueConditions))]()
}

func (c *Cloak) appendConditions(exp ast.Statement) *ast.ExpStatement {
	if c.rand.Intn(2) == 1 {
		return &ast.ExpStatement{
			Operation: ast.OpAnd,
			Left:      exp,
			Right:     c.getTrueCondition(),
		}
	} else {
		return &ast.ExpStatement{
			Operation: ast.OpAnd,
			Left:      c.getTrueCondition(),
			Right:     exp,
		}
	}
}

func (c *Cloak) falseCondition1() *ast.ExpStatement {

	randStr := uuid.NewString()
	startPos := c.rand.Intn(len(randStr) / 2)
	count := c.rand.Intn(len(randStr)/2-1) + 1

	return &ast.ExpStatement{
		Operation: ast.OpEq,
		Left: &ast.ExpStatement{
			Operation: ast.OpPlus,
			Left: &ast.ExpStatement{
				Operation: ast.OpPlus,
				Left: ast.MethodStatement{
					Name: "лев",
					Param: []ast.Statement{
						randStr,
						float64(startPos - 1),
					},
				},
				Right: ast.MethodStatement{
					Name: "Сред",
					Param: []ast.Statement{
						randStr,
						float64(startPos),
						float64(count),
					},
				},
			},
			Right: ast.MethodStatement{
				Name: "прав",
				Param: []ast.Statement{
					randStr,
					float64(len(randStr) - startPos + count),
				},
			},
		},
		Right: randStr,
	}
}

func (c *Cloak) falseCondition2() *ast.ExpStatement {
	exp := c.trueCondition1()
	return exp.Not().(*ast.ExpStatement)
}

func (c *Cloak) falseCondition3() *ast.ExpStatement {
	exp := c.trueCondition2()
	return exp.Not().(*ast.ExpStatement)
}

func (c *Cloak) falseCondition4() *ast.ExpStatement {
	exp := c.trueCondition3()
	return exp.Not().(*ast.ExpStatement)
}

func (c *Cloak) trueCondition1() *ast.ExpStatement {
	return &ast.ExpStatement{
		Operation: ast.OpLt,
		Left:      c.expLess100(),
		Right:     float64(c.rand.Intn(900) + 100),
	}
}

func (c *Cloak) trueCondition2() *ast.ExpStatement {
	return &ast.ExpStatement{
		Operation: ast.OpGt,
		Left:      float64(c.rand.Intn(900) + 100),
		Right:     float64(c.rand.Intn(100)),
	}
}

func (c *Cloak) trueCondition3() *ast.ExpStatement {
	return &ast.ExpStatement{
		Operation: ast.OpGt,
		Left:      float64(c.rand.Intn(900) + 100),
		Right:     float64(c.rand.Intn(100)),
	}
}

func (c *Cloak) trueCondition5() *ast.ExpStatement {
	randStr := uuid.NewString()
	startPos := c.rand.Intn(len(randStr) / 2)
	count := c.rand.Intn(len(randStr)/2-1) + 1

	return &ast.ExpStatement{
		Operation: ast.OpEq,
		Left: &ast.ExpStatement{
			Operation: ast.OpPlus,
			Left: &ast.ExpStatement{
				Operation: ast.OpPlus,
				Left: ast.MethodStatement{
					Name: "лев",
					Param: []ast.Statement{
						randStr,
						float64(startPos - 1),
					},
				},
				Right: ast.MethodStatement{
					Name: "Сред",
					Param: []ast.Statement{
						randStr,
						float64(startPos),
						float64(count),
					},
				},
			},
			Right: ast.MethodStatement{
				Name: "прав",
				Param: []ast.Statement{
					randStr,
					float64(len(randStr) - (startPos + count - 1)),
				},
			},
		},
		Right: randStr,
	}
}

func (c *Cloak) trueCondition4() *ast.ExpStatement {
	exp := c.falseCondition1()
	return exp.Not().(*ast.ExpStatement)
}

func (c *Cloak) falseCondition6() *ast.ExpStatement {
	exp := c.falseCondition2()
	return exp.Not().(*ast.ExpStatement)
}

func (c *Cloak) expLess100() *ast.ExpStatement {
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

// func (c *Cloak) appendRandFunc() string {
// 	// uuid.NewString()
// 	c.oneAppendRandFunc.Do(func() {
// 		c.randFuncName = c.randomString(5)
// 		par1 := c.randomString(5)
// 		par2 := c.randomString(5)
// 		gsch := c.randomString(5)
//
// 		f := ast.FunctionOrProcedure{
// 			Type: ast.PFTypeFunction,
//
// 			Name: c.randFuncName,
// 			Body: []ast.Statement{
// 				&ast.ExpStatement{
// 					Operation: ast.OpEq,
// 					Left:      ast.VarStatement{Name: gsch},
// 					Right: ast.NewObjectStatement{
// 						Constructor: "ГенераторСлучайныхЧисел",
// 						Param: []ast.Statement{
// 							ast.MethodStatement{
// 								Name:  "ТекущаяУниверсальнаяДатаВМиллисекундах",
// 								Param: []ast.Statement{},
// 							},
// 						},
// 					},
// 				},
// 				ast.ReturnStatement{
// 					Param: ast.CallChainStatement{
// 						Unit: ast.MethodStatement{
// 							Name: "СлучайноеЧисло",
// 							Param: []ast.Statement{
// 								ast.VarStatement{Name: par1},
// 								ast.VarStatement{Name: par2},
// 							},
// 						},
// 						Call: ast.VarStatement{
// 							Name: gsch,
// 						},
// 					},
// 				},
// 			},
// 			Export: false,
// 			Params: []ast.ParamStatement{
// 				{Name: par1},
// 				{Name: par2},
// 			},
// 		}
//
// 		// c.moduleBody = append(c.moduleBody, f)
// 		c.a.ModuleStatement.Body = append(c.a.ModuleStatement.Body, f)
// 	})
//
// 	return c.randFuncName
// }

func (c *Cloak) newTernary(trueValue interface{}, depth, trueStep int) ast.TernaryStatement {

	if depth < trueStep {
		depth, trueStep = trueStep, depth
	}

	expression := c.getFalseCondition()
	value := c.fakeValue(trueValue)

	if trueStep == 0 {
		expression = c.getTrueCondition()
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

func (c *Cloak) fakeValue(value interface{}) interface{} {
	switch value.(type) {
	case float64:
		return float64(c.rand.Intn(1000))
	case int:
		return c.rand.Intn(1000)
	case string:
		return c.randomString(10)
	default:
		return value
	}
}

func (c *Cloak) randomString(lenStr int) (result string) {
	charset := []rune("abcdefghijklmnopqrstuvwxyzйцукенгшщзхъфывапролджэячсмитьбю")
	builder := strings.Builder{}

	for builder.Len() < lenStr {
		builder.WriteString(string(charset[c.rand.Intn(len(charset))]))
	}

	return builder.String()
}

func (c *Cloak) obfuscateString(str string, key int32) string {
	var decrypted []rune
	for _, c := range str {
		if unicode.IsLetter(c) {
			decrypted = append(decrypted, c^key)
		} else {
			decrypted = append(decrypted, c)
		}
	}
	return string(decrypted)
}

func (c *Cloak) appendDecodeString() (string, int) {
	c.oneDecodeStringFunc.Do(func() {
		fp, key := c.newDecodeStringFunc()
		c.decodeStringFuncName = fp.Name
		c.decodeKey = key
		c.a.ModuleStatement.Body = append(c.a.ModuleStatement.Body, fp)
	})
	return c.decodeStringFuncName, c.decodeKey
}

func (c *Cloak) newDecodeStringFunc() (*ast.FunctionOrProcedure, int) {
	name := c.randomString(10)
	returnName := c.randomString(5)
	keyVar := c.randomString(5)
	key := c.rand.Intn(90) + 10

	f := &ast.FunctionOrProcedure{
		Type: ast.PFTypeFunction,
		Name: name,
		Body: []ast.Statement{
			&ast.ExpStatement{
				Operation: ast.OpEq,
				Left: ast.VarStatement{
					Name: keyVar,
				},
				Right: c.hideValue(float64(key), 7),
			},
			&ast.ExpStatement{
				Operation: ast.OpEq,
				Left: ast.VarStatement{
					Name: returnName,
				},
				Right: c.hideValue("", 4),
			},
			&ast.LoopStatement{
				Body: []ast.Statement{
					&ast.ExpStatement{
						Operation: ast.OpEq,
						Left: ast.VarStatement{
							Name: "код",
						},
						Right: ast.MethodStatement{
							Name: "КодСимвола",
							Param: []ast.Statement{
								ast.VarStatement{
									Name: "авава",
								},
								ast.VarStatement{
									Name: "_",
								},
							},
						},
					},
					&ast.IfStatement{
						Expression: c.appendConditions(&ast.ExpStatement{
							Operation: ast.OpLt,
							Left: ast.VarStatement{
								Name: "код",
							},
							Right: 65.000000,
						}),
						TrueBlock: []ast.Statement{
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
											ast.VarStatement{
												Name: "код",
											},
										},
									},
								}, 7),
							},
						},
						ElseBlock: []ast.Statement{
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
											ast.MethodStatement{
												Name: "ПобитовоеИли",
												Param: []ast.Statement{
													ast.MethodStatement{
														Name: "ПобитовоеИНе",
														Param: []ast.Statement{
															ast.VarStatement{
																Name: "код",
															},
															ast.VarStatement{
																Name: keyVar,
															},
														},
													},
													ast.MethodStatement{
														Name: "ПобитовоеИНе",
														Param: []ast.Statement{
															ast.VarStatement{
																Name: keyVar,
															},
															ast.VarStatement{
																Name: "код",
															},
														},
													},
												},
											},
										},
									},
								}, 5),
							},
						},
					},
				},
				To: ast.MethodStatement{
					Name: "СтрДлина",
					Param: []ast.Statement{
						ast.VarStatement{
							Name: "авава",
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
			ast.ReturnStatement{
				Param: ast.VarStatement{
					Name: returnName,
				},
			},
		},
		Params: []ast.ParamStatement{
			{Name: "авава"},
		},
	}

	return f, key
}
