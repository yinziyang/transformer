package transformer_test

import (
	"reflect"
	"testing"

	"github.com/yinziyang/transformer"
	"github.com/yinziyang/transformer/bert"
)

// With model name
func TestModelFromPretrained_ModelName(t *testing.T) {
	modelName := "bert-base-uncased"
	var config *bert.BertConfig = new(bert.BertConfig)
	err := transformer.LoadConfig(config, modelName, nil)
	if err != nil {
		t.Error(err)
	}

	wantVocabSize := int64(30522)
	gotVocabSize := config.VocabSize

	if !reflect.DeepEqual(wantVocabSize, gotVocabSize) {
		t.Errorf("Want: %v\n", wantVocabSize)
		t.Errorf("Got: %v\n", gotVocabSize)
	}
}

// With local file

/*
 * // No custom params
 * func TestModelFromPretrained(t *testing.T) {
 *   // bertURL := transformer.AllPretrainedConfigs["bert-base-uncased"]
 *   url := "https://s3.amazonaws.com/models.huggingface.co/bert/bert-base-uncased-config.json"
 *
 *   var config *bert.BertConfig = new(bert.BertConfig)
 *   err := transformer.LoadConfig(config, url, nil)
 *   if err != nil {
 *     t.Error(err)
 *   }
 *
 *   wantVocabSize := int64(30522)
 *   gotVocabSize := config.VocabSize
 *
 *   if !reflect.DeepEqual(wantVocabSize, gotVocabSize) {
 *     t.Errorf("Want: %v\n", wantVocabSize)
 *     t.Errorf("Got: %v\n", gotVocabSize)
 *   }
 *
 * }
 *
 * // With custom params
 * func TestModelFromPretrained_CustomParams(t *testing.T) {
 *   url := "https://s3.amazonaws.com/models.huggingface.co/bert/bert-base-uncased-config.json"
 *
 *   params := map[string]interface{}{
 *     "VocabSize": int64(2000),
 *     "NumLabels": int64(4),
 *   }
 *
 *   var config *bert.BertConfig = new(bert.BertConfig)
 *   err := transformer.LoadConfig(config, url, params)
 *   if err != nil {
 *     t.Error(err)
 *   }
 *
 *   wantVocabSize := int64(2000)
 *   gotVocabSize := config.VocabSize
 *
 *   if !reflect.DeepEqual(wantVocabSize, gotVocabSize) {
 *     t.Errorf("Want: %v\n", wantVocabSize)
 *     t.Errorf("Got: %v\n", gotVocabSize)
 *   }
 *
 *   wantNumLabels := int64(4)
 *   gotNumLabels := config.NumLabels
 *
 *   if !reflect.DeepEqual(wantNumLabels, gotNumLabels) {
 *     t.Errorf("Want: %v\n", wantNumLabels)
 *     t.Errorf("Got: %v\n", gotNumLabels)
 *   }
 * } */
