package obfuscator

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/knetic/govaluate"
	"github.com/stretchr/testify/assert"
)

func TestObfuscate(t *testing.T) {

	code := `Процедура ЗагрузитьКонтактнуюИнформацию(ДанныеXDTO, ПолученныеДанные, ГруппаВидовКИ)
	Если Не (ДанныеXDTO.Свойство("КонтактнаяИнформация")
		И ЗначениеЗаполнено(ДанныеXDTO.КонтактнаяИнформация)) Тогда
		Возврат;
	КонецЕсли;
		
	Для Каждого СтрокаXDTO Из ДанныеXDTO.КонтактнаяИнформация Цикл
		ВидКИСтрокой   = СокрЛП(СтрокаXDTO.ВидКонтактнойИнформации.Значение);
		НаименованиеКИ = СокрЛП(СтрокаXDTO.НаименованиеКонтактнойИнформации);
		
		ТекВидКИ = ВидКонтактнойИнформацииИзСтроки(ВидКИСтрокой, НаименованиеКИ, ГруппаВидовКИ);
		
		Если Не ЗначениеЗаполнено(ТекВидКИ) Тогда
			Продолжить;
		КонецЕсли;
	
		УправлениеКонтактнойИнформацией.ДобавитьКонтактнуюИнформацию(ПолученныеДанные, СокрЛП(СтрокаXDTO.ЗначенияПолей), ТекВидКИ);
	КонецЦикла;
КонецПроцедуры

`

	obf := NewObfuscatory(context.Background(), Config{
		RepExpByTernary:  true,
		RepLoopByGoto:    true,
		RepExpByInvoke:   true,
		HideString:       true,
		ChangeConditions: true,
		AppendGarbage:    true,
		// ShuffleExpressions: true,
	})
	obCode, err := obf.Obfuscate(code)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(obCode)
}

func TestObfuscateLoop(t *testing.T) {

	code := `&НаСервереБезКонтекста
			Процедура Команда1НаСервере()     
				Для а = 0 По 100 Цикл
					Сообщить(а);	
				КонецЦикла;
				
				б = 0;
				Пока б < 100 Цикл
					Сообщить(б);
					б = б+1;
				КонецЦикла;
				
				fd = "dssdfdf";
				Для Каждого а Из Чтото Цикл
					Сообщить(а);	
				КонецЦикла;

			 КонецПроцедуры`

	obf := NewObfuscatory(context.Background(), Config{RepLoopByGoto: false})
	obCode, err := obf.Obfuscate(code)
	if err != nil {
		fmt.Println(err)
		return
	}

	// должны быть равны
	assert.Equal(t, true, compareHashes(code, obCode))

	obf = NewObfuscatory(context.Background(), Config{RepLoopByGoto: true})
	obCode, err = obf.Obfuscate(code)
	if err != nil {
		fmt.Println(err)
		return
	}

	// не должны быть равны
	assert.Equal(t, false, compareHashes(code, obCode))
}

func TestShuffleExp(t *testing.T) {
	//
	// 	code := `&НаСервереБезКонтекста
	// 			Процедура Команда1НаСервере()
	//
	// 			а = 1;
	// 			Сообщить(а);
	// 			а = а +1;
	// 			Сообщить(а);
	// 			а = а +1;
	// 			Сообщить(а);
	// 			а = а +1;
	// 			Сообщить(а);
	//
	// Если Истина Тогда
	// а = а +1;
	// 			Сообщить(а);
	// а = а +1;
	// 			Сообщить(а);
	// КонецЕсли;
	// а = а +1;
	// 			Сообщить(а);
	// а = а +1;
	// 			Сообщить(а);
	//
	// 			 КонецПроцедуры`
	//
	// 	obf := NewObfuscatory(context.Background(), Config{ShuffleExpressions: true})
	// 	obCode, err := obf.Obfuscate(code)
	// 	if err != nil {
	// 		fmt.Println(err)
	// 		return
	// 	}
	//
	// 	fmt.Println(obCode)
}

func TestGenCondition(t *testing.T) {
	ctx, _ := context.WithTimeout(context.Background(), time.Millisecond*500)
	obf := NewObfuscatory(ctx, Config{})

	for c := range obf.falseCondition {
		expression, _ := govaluate.NewEvaluableExpression(c)
		result, _ := expression.Evaluate(nil)
		if v, ok := result.(bool); v && ok {
			t.Fatal(c, "expression must be false")
		}
	}

	for c := range obf.trueCondition {
		expression, _ := govaluate.NewEvaluableExpression(c)
		result, _ := expression.Evaluate(nil)
		if v, ok := result.(bool); v && !ok {
			t.Fatal(c, "expression must be true")
		}
	}
}

func compareHashes(str1, str2 string) bool {
	str1 = strings.ReplaceAll(str1, " ", "")
	str1 = strings.ReplaceAll(str1, "\t", "")
	str1 = strings.ReplaceAll(str1, "\n", "")

	str2 = strings.ReplaceAll(str2, " ", "")
	str2 = strings.ReplaceAll(str2, "\t", "")
	str2 = strings.ReplaceAll(str2, "\n", "")

	hash1 := sha256.Sum256([]byte(strings.ToLower(str1)))
	hash2 := sha256.Sum256([]byte(strings.ToLower(str2)))

	return hash1 == hash2
}
