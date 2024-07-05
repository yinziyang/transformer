package bert_test

import (
	"fmt"
	"log"
	"reflect"
	"testing"

	"github.com/sugarme/gotch"
	"github.com/sugarme/gotch/nn"
	"github.com/sugarme/gotch/pickle"
	"github.com/sugarme/gotch/ts"
	"github.com/sugarme/tokenizer"
	"github.com/sugarme/tokenizer/model/wordpiece"
	"github.com/sugarme/tokenizer/normalizer"
	"github.com/sugarme/tokenizer/pretokenizer"
	"github.com/sugarme/tokenizer/processor"

	"github.com/yinziyang/transformer/bert"
	"github.com/yinziyang/transformer/util"
)

func getBertTokenizer(vocabFile string) (retVal *tokenizer.Tokenizer) {
	model, err := wordpiece.NewWordPieceFromFile(vocabFile, "[UNK]")
	if err != nil {
		log.Fatal(err)
	}

	tk := tokenizer.NewTokenizer(model)

	bertNormalizer := normalizer.NewBertNormalizer(true, true, true, true)
	tk.WithNormalizer(bertNormalizer)

	bertPreTokenizer := pretokenizer.NewBertPreTokenizer()
	tk.WithPreTokenizer(bertPreTokenizer)

	var specialTokens []tokenizer.AddedToken
	specialTokens = append(specialTokens, tokenizer.NewAddedToken("[MASK]", true))

	tk.AddSpecialTokens(specialTokens)

	sepId, ok := tk.TokenToId("[SEP]")
	if !ok {
		log.Fatalf("Cannot find ID for [SEP] token.\n")
	}
	sep := processor.PostToken{Id: sepId, Value: "[SEP]"}

	clsId, ok := tk.TokenToId("[CLS]")
	if !ok {
		log.Fatalf("Cannot find ID for [CLS] token.\n")
	}
	cls := processor.PostToken{Id: clsId, Value: "[CLS]"}

	postProcess := processor.NewBertProcessing(sep, cls)
	tk.WithPostProcessor(postProcess)

	return tk
}

func TestBertForMaskedLM(t *testing.T) {
	// Config
	config := new(bert.BertConfig)

	configFile, err := util.CachedPath("bert-base-uncased", "config.json")
	if err != nil {
		log.Fatal(err)
	}

	err = config.Load(configFile, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Model
	device := gotch.CPU
	vs := nn.NewVarStore(device)

	model, err := bert.NewBertForMaskedLM(vs.Root(), config)
	if err != nil {
		log.Fatal(err)
	}
	modelFile, err := util.CachedPath("bert-base-uncased", "pytorch_model.bin")
	if err != nil {
		log.Fatal(err)
	}

	err = pickle.LoadAll(vs, modelFile)
	if err != nil {
		t.Error(err)
	}

	vocabFile, err := util.CachedPath("bert-base-uncased", "vocab.txt")
	tk := getBertTokenizer(vocabFile)
	sentence1 := "Looks like one [MASK] is missing"
	sentence2 := "It was a very nice and [MASK] day"

	var input []tokenizer.EncodeInput
	input = append(input, tokenizer.NewSingleEncodeInput(tokenizer.NewInputSequence(sentence1)))
	input = append(input, tokenizer.NewSingleEncodeInput(tokenizer.NewInputSequence(sentence2)))

	encodings, err := tk.EncodeBatch(input, true)
	if err != nil {
		log.Fatal(err)
	}

	var maxLen int = 0
	for _, en := range encodings {
		if len(en.Ids) > maxLen {
			maxLen = len(en.Ids)
		}
	}

	var tensors []ts.Tensor
	for _, en := range encodings {
		var tokInput []int64 = make([]int64, maxLen)
		for i := 0; i < len(en.Ids); i++ {
			tokInput[i] = int64(en.Ids[i])
		}

		tensors = append(tensors, *ts.TensorFrom(tokInput))
	}

	inputTensor := ts.MustStack(tensors, 0).MustTo(device, true)

	var output *ts.Tensor
	ts.NoGrad(func() {
		output, _, _ = model.ForwardT(inputTensor, ts.None, ts.None, ts.None, ts.None, ts.None, ts.None, false)
	})

	index1 := output.MustGet(0).MustGet(4).MustArgmax([]int64{0}, false, false).Int64Values()[0]
	index2 := output.MustGet(1).MustGet(7).MustArgmax([]int64{0}, false, false).Int64Values()[0]

	got1, ok := tk.IdToToken(int(index1))
	if !ok {
		fmt.Printf("Cannot find a corresponding word for the given id (%v) in vocab.\n", index1)
	}
	want1 := "person"

	if !reflect.DeepEqual(want1, got1) {
		t.Errorf("Want: '%v'\n", want1)
		t.Errorf("Got '%v'\n", got1)
	}

	got2, ok := tk.IdToToken(int(index2))
	if !ok {
		fmt.Printf("Cannot find a corresponding word for the given id (%v) in vocab.\n", index2)
	}
	want2 := "pleasant"

	if !reflect.DeepEqual(want2, got2) {
		t.Errorf("Want: '%v'\n", want2)
		t.Errorf("Got '%v'\n", got2)
	}
}

func TestBertForSequenceClassification(t *testing.T) {
	device := gotch.CPU
	vs := nn.NewVarStore(device)

	configFile, err := util.CachedPath("bert-base-uncased", "config.json")
	if err != nil {
		t.Error(err)
	}
	config, err := bert.ConfigFromFile(configFile)
	if err != nil {
		t.Error(err)
	}

	var dummyLabelMap map[int64]string = make(map[int64]string)
	dummyLabelMap[0] = "positive"
	dummyLabelMap[1] = "negative"
	dummyLabelMap[3] = "neutral"

	config.Id2Label = dummyLabelMap
	config.OutputAttentions = true
	config.OutputHiddenStates = true
	model := bert.NewBertForSequenceClassification(vs.Root(), config)

	vocabFile, err := util.CachedPath("bert-base-uncased", "vocab.txt")
	tk := getBertTokenizer(vocabFile)

	// Define input
	sentence1 := "Looks like one thing is missing"
	sentence2 := `It's like comparing oranges to apples`
	var input []tokenizer.EncodeInput
	input = append(input, tokenizer.NewSingleEncodeInput(tokenizer.NewInputSequence(sentence1)))
	input = append(input, tokenizer.NewSingleEncodeInput(tokenizer.NewInputSequence(sentence2)))
	encodings, err := tk.EncodeBatch(input, true)
	if err != nil {
		log.Fatal(err)
	}

	// Find max length of token Ids from slice of encodings
	var maxLen int = 0
	for _, en := range encodings {
		if len(en.Ids) > maxLen {
			maxLen = len(en.Ids)
		}
	}

	var tensors []ts.Tensor
	for _, en := range encodings {
		var tokInput []int64 = make([]int64, maxLen)
		for i := 0; i < len(en.Ids); i++ {
			tokInput[i] = int64(en.Ids[i])
		}

		tensors = append(tensors, *ts.TensorFrom(tokInput))
	}

	inputTensor := ts.MustStack(tensors, 0).MustTo(device, true)

	var (
		output                         *ts.Tensor
		allHiddenStates, allAttentions []ts.Tensor
	)

	ts.NoGrad(func() {
		output, allHiddenStates, allAttentions = model.ForwardT(inputTensor, ts.None, ts.None, ts.None, ts.None, false)
	})

	fmt.Printf("output size: %v\n", output.MustSize())
	gotOuputSize := output.MustSize()
	wantOuputSize := []int64{2, 3}
	if !reflect.DeepEqual(wantOuputSize, gotOuputSize) {
		t.Errorf("Want: %v\n", wantOuputSize)
		t.Errorf("Got: %v\n", gotOuputSize)
	}

	numHiddenLayers := int(config.NumHiddenLayers)

	if !reflect.DeepEqual(numHiddenLayers, len(allHiddenStates)) {
		t.Errorf("Want num of allHiddenStates: %v\n", numHiddenLayers)
		t.Errorf("Got num of allHiddenStates: %v\n", len(allHiddenStates))
	}

	if !reflect.DeepEqual(numHiddenLayers, len(allAttentions)) {
		t.Errorf("Want num of allAttentions: %v\n", numHiddenLayers)
		t.Errorf("Got num of allAttentions: %v\n", len(allAttentions))
	}
}

func TestBertForMultipleChoice(t *testing.T) {
	device := gotch.CPU
	vs := nn.NewVarStore(device)

	configFile, err := util.CachedPath("bert-base-uncased", "config.json")
	if err != nil {
		t.Error(err)
	}
	config, err := bert.ConfigFromFile(configFile)
	if err != nil {
		t.Error(err)
	}

	config.OutputAttentions = true
	config.OutputHiddenStates = true
	model := bert.NewBertForMultipleChoice(vs.Root(), config)

	vocabFile, err := util.CachedPath("bert-base-uncased", "vocab.txt")
	tk := getBertTokenizer(vocabFile)

	// Define input
	sentence1 := "Looks like one thing is missing"
	sentence2 := `It's like comparing oranges to apples`
	var input []tokenizer.EncodeInput
	input = append(input, tokenizer.NewSingleEncodeInput(tokenizer.NewInputSequence(sentence1)))
	input = append(input, tokenizer.NewSingleEncodeInput(tokenizer.NewInputSequence(sentence2)))
	encodings, err := tk.EncodeBatch(input, true)
	if err != nil {
		log.Fatal(err)
	}

	// Find max length of token Ids from slice of encodings
	var maxLen int = 0
	for _, en := range encodings {
		if len(en.Ids) > maxLen {
			maxLen = len(en.Ids)
		}
	}

	var tensors []ts.Tensor
	for _, en := range encodings {
		var tokInput []int64 = make([]int64, maxLen)
		for i := 0; i < len(en.Ids); i++ {
			tokInput[i] = int64(en.Ids[i])
		}

		tensors = append(tensors, *ts.TensorFrom(tokInput))
	}

	inputTensor := ts.MustStack(tensors, 0).MustTo(device, true).MustUnsqueeze(0, true)

	var (
		output                         *ts.Tensor
		allHiddenStates, allAttentions []ts.Tensor
	)

	ts.NoGrad(func() {
		output, allHiddenStates, allAttentions = model.ForwardT(inputTensor, ts.None, ts.None, ts.None, false)
	})

	fmt.Printf("output size: %v\n", output.MustSize())
	gotOuputSize := output.MustSize()
	wantOuputSize := []int64{1, 2}
	if !reflect.DeepEqual(wantOuputSize, gotOuputSize) {
		t.Errorf("Want: %v\n", wantOuputSize)
		t.Errorf("Got: %v\n", gotOuputSize)
	}

	numHiddenLayers := int(config.NumHiddenLayers)

	if !reflect.DeepEqual(numHiddenLayers, len(allHiddenStates)) {
		t.Errorf("Want num of allHiddenStates: %v\n", numHiddenLayers)
		t.Errorf("Got num of allHiddenStates: %v\n", len(allHiddenStates))
	}

	if !reflect.DeepEqual(numHiddenLayers, len(allAttentions)) {
		t.Errorf("Want num of allAttentions: %v\n", numHiddenLayers)
		t.Errorf("Got num of allAttentions: %v\n", len(allAttentions))
	}
}

func TestBertForTokenClassification(t *testing.T) {
	device := gotch.CPU
	vs := nn.NewVarStore(device)

	configFile, err := util.CachedPath("bert-base-uncased", "config.json")
	if err != nil {
		t.Error(err)
	}
	config, err := bert.ConfigFromFile(configFile)
	if err != nil {
		t.Error(err)
	}

	var dummyLabelMap map[int64]string = make(map[int64]string)
	dummyLabelMap[0] = "O"
	dummyLabelMap[1] = "LOC"
	dummyLabelMap[2] = "PER"
	dummyLabelMap[3] = "ORG"

	config.Id2Label = dummyLabelMap
	config.OutputAttentions = true
	config.OutputHiddenStates = true
	model := bert.NewBertForTokenClassification(vs.Root(), config)

	vocabFile, err := util.CachedPath("bert-base-uncased", "vocab.txt")
	tk := getBertTokenizer(vocabFile)

	// Define input
	sentence1 := "Looks like one thing is missing"
	sentence2 := `It's like comparing oranges to apples`
	var input []tokenizer.EncodeInput
	input = append(input, tokenizer.NewSingleEncodeInput(tokenizer.NewInputSequence(sentence1)))
	input = append(input, tokenizer.NewSingleEncodeInput(tokenizer.NewInputSequence(sentence2)))
	encodings, err := tk.EncodeBatch(input, true)
	if err != nil {
		log.Fatal(err)
	}

	// Find max length of token Ids from slice of encodings
	var maxLen int = 0
	for _, en := range encodings {
		if len(en.Ids) > maxLen {
			maxLen = len(en.Ids)
		}
	}

	var tensors []ts.Tensor
	for _, en := range encodings {
		var tokInput []int64 = make([]int64, maxLen)
		for i := 0; i < len(en.Ids); i++ {
			tokInput[i] = int64(en.Ids[i])
		}

		tensors = append(tensors, *ts.TensorFrom(tokInput))
	}

	inputTensor := ts.MustStack(tensors, 0).MustTo(device, true)

	var (
		output                         *ts.Tensor
		allHiddenStates, allAttentions []ts.Tensor
	)

	ts.NoGrad(func() {
		output, allHiddenStates, allAttentions = model.ForwardT(inputTensor, ts.None, ts.None, ts.None, ts.None, false)
	})

	fmt.Printf("output size: %v\n", output.MustSize())
	gotOuputSize := output.MustSize()
	wantOuputSize := []int64{2, 11, 4}
	if !reflect.DeepEqual(wantOuputSize, gotOuputSize) {
		t.Errorf("Want: %v\n", wantOuputSize)
		t.Errorf("Got: %v\n", gotOuputSize)
	}

	numHiddenLayers := int(config.NumHiddenLayers)

	if !reflect.DeepEqual(numHiddenLayers, len(allHiddenStates)) {
		t.Errorf("Want num of allHiddenStates: %v\n", numHiddenLayers)
		t.Errorf("Got num of allHiddenStates: %v\n", len(allHiddenStates))
	}

	if !reflect.DeepEqual(numHiddenLayers, len(allAttentions)) {
		t.Errorf("Want num of allAttentions: %v\n", numHiddenLayers)
		t.Errorf("Got num of allAttentions: %v\n", len(allAttentions))
	}
}

func TestBertForQuestionAnswering(t *testing.T) {
	device := gotch.CPU
	vs := nn.NewVarStore(device)

	configFile, err := util.CachedPath("bert-base-uncased", "config.json")
	if err != nil {
		t.Error(err)
	}
	config, err := bert.ConfigFromFile(configFile)
	if err != nil {
		t.Error(err)
	}

	config.OutputAttentions = true
	config.OutputHiddenStates = true
	model := bert.NewForBertQuestionAnswering(vs.Root(), config)

	vocabFile, err := util.CachedPath("bert-base-uncased", "vocab.txt")
	tk := getBertTokenizer(vocabFile)

	// Define input
	sentence1 := "Looks like one thing is missing"
	sentence2 := `It's like comparing oranges to apples`
	var input []tokenizer.EncodeInput
	input = append(input, tokenizer.NewSingleEncodeInput(tokenizer.NewInputSequence(sentence1)))
	input = append(input, tokenizer.NewSingleEncodeInput(tokenizer.NewInputSequence(sentence2)))
	encodings, err := tk.EncodeBatch(input, true)
	if err != nil {
		log.Fatal(err)
	}

	// Find max length of token Ids from slice of encodings
	var maxLen int = 0
	for _, en := range encodings {
		if len(en.Ids) > maxLen {
			maxLen = len(en.Ids)
		}
	}

	var tensors []ts.Tensor
	for _, en := range encodings {
		var tokInput []int64 = make([]int64, maxLen)
		for i := 0; i < len(en.Ids); i++ {
			tokInput[i] = int64(en.Ids[i])
		}

		tensors = append(tensors, *ts.TensorFrom(tokInput))
	}

	inputTensor := ts.MustStack(tensors, 0).MustTo(device, true)

	var (
		startScores, endScores         *ts.Tensor
		allHiddenStates, allAttentions []ts.Tensor
	)

	ts.NoGrad(func() {
		startScores, endScores, allHiddenStates, allAttentions = model.ForwardT(inputTensor, ts.None, ts.None, ts.None, ts.None, false)
	})

	gotStartScoresSize := startScores.MustSize()
	wantStartScoresSize := []int64{2, 11}
	if !reflect.DeepEqual(wantStartScoresSize, gotStartScoresSize) {
		t.Errorf("Want: %v\n", wantStartScoresSize)
		t.Errorf("Got: %v\n", gotStartScoresSize)
	}

	gotEndScoresSize := endScores.MustSize()
	wantEndScoresSize := []int64{2, 11}
	if !reflect.DeepEqual(wantEndScoresSize, gotEndScoresSize) {
		t.Errorf("Want: %v\n", wantEndScoresSize)
		t.Errorf("Got: %v\n", gotEndScoresSize)
	}

	numHiddenLayers := int(config.NumHiddenLayers)

	if !reflect.DeepEqual(numHiddenLayers, len(allHiddenStates)) {
		t.Errorf("Want num of allHiddenStates: %v\n", numHiddenLayers)
		t.Errorf("Got num of allHiddenStates: %v\n", len(allHiddenStates))
	}

	if !reflect.DeepEqual(numHiddenLayers, len(allAttentions)) {
		t.Errorf("Want num of allAttentions: %v\n", numHiddenLayers)
		t.Errorf("Got num of allAttentions: %v\n", len(allAttentions))
	}
}
