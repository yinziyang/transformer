package bert

import (
	"fmt"

	"github.com/sugarme/gotch"
	"github.com/sugarme/gotch/nn"
	"github.com/sugarme/gotch/ts"

	"github.com/yinziyang/transformer/util"
)

// BertEmbedding defines interface for BertModel or RoBertaModel.
type BertEmbedding interface {
	ForwardT(inputIds, tokenTypeIds, positionIds, inputEmbeds *ts.Tensor, train bool) (*ts.Tensor, error)
}

type BertEmbeddings struct {
	WordEmbeddings      *nn.Embedding
	PositionEmbeddings  *nn.Embedding
	TokenTypeEmbeddings *nn.Embedding
	LayerNorm           *nn.LayerNorm
	Dropout             *util.Dropout
}

// NewBertEmbeddings builds a new BertEmbeddings
func NewBertEmbeddings(p *nn.Path, config *BertConfig, changeNameOpt ...bool) *BertEmbeddings {
	changeName := true
	if len(changeNameOpt) > 0 {
		changeName = changeNameOpt[0]
	}
	embeddingConfig := nn.DefaultEmbeddingConfig()
	embeddingConfig.PaddingIdx = 0

	wEmbedPath := p.Sub("word_embeddings")
	wordEmbeddings := nn.NewEmbedding(wEmbedPath, config.VocabSize, config.HiddenSize, embeddingConfig)

	posEmbedPath := p.Sub("position_embeddings")
	positionEmbeddings := nn.NewEmbedding(posEmbedPath, config.MaxPositionEmbeddings, config.HiddenSize, embeddingConfig)

	ttEmbedPath := p.Sub("token_type_embeddings")
	tokenTypeEmbeddings := nn.NewEmbedding(ttEmbedPath, config.TypeVocabSize, config.HiddenSize, embeddingConfig)

	layerNormConfig := nn.DefaultLayerNormConfig()
	if changeName {
		layerNormConfig.WsName = "gamma"
		layerNormConfig.BsName = "beta"
	}
	layerNormConfig.Eps = 1e-12

	lnPath := p.Sub("LayerNorm")
	layerNorm := nn.NewLayerNorm(lnPath, []int64{config.HiddenSize}, layerNormConfig)

	dropout := util.NewDropout(config.HiddenDropoutProb)

	return &BertEmbeddings{wordEmbeddings, positionEmbeddings, tokenTypeEmbeddings, layerNorm, dropout}
}

// ForwardT implements BertEmbedding interface, passes throught the embedding layer
func (be *BertEmbeddings) ForwardT(inputIds, tokenTypeIds, positionIds, inputEmbeds *ts.Tensor, train bool) (retVal *ts.Tensor, err error) {

	var (
		inputEmbeddings *ts.Tensor
		inputShape      []int64
	)

	if inputIds.MustDefined() {
		if inputEmbeds.MustDefined() {
			err = fmt.Errorf("Only one of input Ids or input embeddings may be set.")
			return retVal, err
		} else {
			inputEmbeddings = inputIds.ApplyT(be.WordEmbeddings, train)
			inputShape = inputIds.MustSize()
		}
	} else {
		if inputEmbeds.MustDefined() {
			inputEmbeddings = inputEmbeds
			size := inputEmbeds.MustSize()
			inputShape = []int64{size[0], size[1]}
		} else {
			err = fmt.Errorf("Only one of input Ids or input embeddings may be set.")
			return retVal, err
		}
	}

	seqLength := inputEmbeddings.MustSize()[1]

	var posIds *ts.Tensor
	if positionIds.MustDefined() {
		posIds = positionIds
	} else {
		tmp1 := ts.MustArange(ts.IntScalar(seqLength), gotch.Int64, inputEmbeddings.MustDevice())
		tmp2 := tmp1.MustUnsqueeze(0, true)
		posIds = tmp2.MustExpand(inputShape, true, true)
	}

	var tokTypeIds *ts.Tensor
	if tokenTypeIds.MustDefined() {
		tokTypeIds = tokenTypeIds
	} else {
		tokTypeIds = ts.MustZeros(inputShape, gotch.Int64, inputEmbeddings.MustDevice())
	}

	posEmbeddings := posIds.Apply(be.PositionEmbeddings)
	posIds.MustDrop()
	tokEmbeddings := tokTypeIds.Apply(be.TokenTypeEmbeddings)
	tokTypeIds.MustDrop()

	input := inputEmbeddings.MustAdd(posEmbeddings, true)
	posEmbeddings.MustDrop()
	input.MustAdd_(tokEmbeddings)
	tokEmbeddings.MustDrop()

	retTmp1 := input.Apply(be.LayerNorm)
	input.MustDrop()
	retVal = retTmp1.ApplyT(be.Dropout, train)
	retTmp1.MustDrop()

	return retVal, nil
}
