package transformer_test

import (
	"fmt"
	"log"

	"github.com/yinziyang/transformer"
	"github.com/yinziyang/transformer/bert"
)

func ExampleLoadConfig() {
	modelNameOrPath := "bert-base-uncased"
	var config bert.BertConfig
	err := transformer.LoadConfig(&config, modelNameOrPath, nil)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(config.VocabSize)

	// Output:
	// 30522
}
