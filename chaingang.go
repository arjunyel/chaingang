package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	bittrex "github.com/toorop/go-bittrex"
)

type balances struct {
	lock     sync.RWMutex
	balances map[string]decimal.Decimal
}

type Coin struct {
	Name          string
	Relationships map[string]Relationship
}

type Relationship struct {
	Ask       decimal.Decimal
	Bid       decimal.Decimal
	Last      decimal.Decimal
	Timestamp string
}

type summary struct {
	Direct     decimal.Decimal
	Gain       decimal.Decimal
	Indirect   decimal.Decimal
	InputCoin  string
	OutputCoin string
	Quantity   decimal.Decimal
	Vessel     string
}

type parentCoin struct {
	Name       string
	Btc        decimal.Decimal
	Eth        decimal.Decimal
	Usdt       decimal.Decimal
	ChildCoins []string
}

type childCoin struct {
	Name string
	Btc  decimal.Decimal
	Eth  decimal.Decimal
	//TODO add usdt
	ParentCoins []string
	Summary     summary
}

/*
type summary struct {
	ChildToBtc     decimal.Decimal
	ChildToEth     decimal.Decimal
	IndirectToEth  decimal.Decimal
	IndirectToBtc  decimal.Decimal
	DirectToEth    decimal.Decimal
	DirectToBtc    decimal.Decimal
	DiffEth        decimal.Decimal
	DiffBtc        decimal.Decimal
	DiffEthUsdt    decimal.Decimal
	DiffBtcUsdt    decimal.Decimal
	DiffBtcPerUsdt decimal.Decimal
	DiffEthPerUsdt decimal.Decimal
}
*/
// Global Variables
var (
	acctBalance = &balances{
		lock:     sync.RWMutex{},
		balances: make(map[string]decimal.Decimal),
	}
	transactionFee = decimal.NewFromFloat(.0025)
	parentCoins    = map[string]*parentCoin{}
	childCoins     = map[string]*childCoin{}
	coins          = map[string]*Coin{}
	validOrigins   = map[string]map[string]decimal.Decimal{
		"Bittrex": map[string]decimal.Decimal{
			"BTC":  decimal.NewFromFloat(0.0072),
			"ETH":  decimal.NewFromFloat(0.00071),
			"USDT": decimal.NewFromFloat(100),
		},
	}
	exchangeName = "Bittrex"
	summaries    map[string]map[string][]summary
)

/* ******************************************************************
 * API Calls
 * *****************************************************************/
func updateMarketSummaries(bittrexClient *bittrex.Bittrex) ([]bittrex.MarketSummary, error) {
	marketSummaries, err := bittrexClient.GetMarketSummaries()
	return marketSummaries, err
}

func (b *balances) updateAccountBalances(bittrexClient *bittrex.Bittrex) error {
	balances, err := bittrexClient.GetBalances()
	if err != nil {

		return err
	}
	zero := decimal.NewFromFloat(0.0)
	b.lock.Lock()
	for _, bal := range balances {
		if bal.Available.GreaterThan(zero) {
			b.balances[bal.Currency] = bal.Available
		}
	}
	b.lock.Unlock()
	return nil
}

/* ******************************************************************
 * Populate Metrics for Child and Parent Coins
 * *****************************************************************/
func createCoins(marketSummaries []bittrex.MarketSummary) {
	for _, marketSummary := range marketSummaries {
		relationshipName := strings.Split(marketSummary.MarketName, "-")[0]
		coinName := strings.Split(marketSummary.MarketName, "-")[1]

		_, coinExists := coins[coinName]
		zero := decimal.NewFromFloat(0.0)
		if !marketSummary.Ask.Equal(zero) && !marketSummary.Bid.Equal(zero) && !marketSummary.Last.Equal(zero) {
			if !coinExists {
				coins[coinName] = &Coin{
					Name:          coinName,
					Relationships: make(map[string]Relationship),
				}
			}
			coins[coinName].Relationships[relationshipName] = Relationship{
				Ask:  marketSummary.Ask,
				Bid:  marketSummary.Bid,
				Last: marketSummary.Last,
			}
		}
	}

	for originName := range validOrigins[exchangeName] {
		_, marketHasCoin := coins[originName]
		if !marketHasCoin {
			coins[originName] = &Coin{
				Name:          originName,
				Relationships: make(map[string]Relationship),
			}
		}
	}
}

func populateCoins() {
	for coinName, coinValue := range coins {
		for originName := range validOrigins[exchangeName] {
			if originName != coinName {
				_, hasRelationship := coinValue.Relationships[originName]
				if !hasRelationship && isValidRelationship(exchangeName, coinName) {
					fmt.Printf("%v : %v\n", coinName, originName)
					ask := decimal.NewFromFloat(0)
					bid := decimal.NewFromFloat(0)
					last := decimal.NewFromFloat(0)
					if coins[originName].Relationships[coinName].Ask != decimal.NewFromFloat(0) {
						ask = decimal.NewFromFloat(1).Div(coins[originName].Relationships[coinName].Ask)
					}
					if coins[originName].Relationships[coinName].Bid != decimal.NewFromFloat(0) {
						bid = decimal.NewFromFloat(1).Div(coins[originName].Relationships[coinName].Bid)
					}
					if coins[originName].Relationships[coinName].Last != decimal.NewFromFloat(0) {
						last = decimal.NewFromFloat(1).Div(coins[originName].Relationships[coinName].Last)
					}
					coinValue.Relationships[originName] = Relationship{

						Ask:  ask,
						Bid:  bid,
						Last: last,
					}
				}
			}
		}
	}
}

func createSummaries(bittrexClient *bittrex.Bittrex) {

	err := acctBalance.updateAccountBalances(bittrexClient)
	if err != nil {
		fmt.Println(err)
		return
	} else {
		for originName := range validOrigins[exchangeName] {
			availableOrigin, accHasOrigin := acctBalance.get(originName)
			maxOriginStake := validOrigins[exchangeName][originName]
			originStake := maxOriginStake
			if originStake.GreaterThan(availableOrigin) {
				originStake = availableOrigin
			}
			if accHasOrigin {
				summaries[originName] = make(map[string][]summary)
				for otherOriginName := range validOrigins[exchangeName] {
					if originName != otherOriginName && originName != "USDT" {
						summaries[originName][otherOriginName] = make([]summary, 0)
						directAsk, _, _, _ := convert(originName, otherOriginName, originStake)
						//	fmt.Printf("Direct Ask %v -> %v : %v\n", marketName, otherMarketName, directAsk)
						for coinName := range coins {
							_, marketToCoinBid, _, marketToCoinConvertable := convert(originName, coinName, originStake)
							coinToOtherAsk, _, _, coinToOtherConvertable := convert(coinName, otherOriginName, marketToCoinBid)
							_, otherToMarketBid, _, otherToMarketConvertable := convert(otherOriginName, originName, coinToOtherAsk)
							if marketToCoinConvertable && coinToOtherConvertable && otherToMarketConvertable {
								summaries[originName][otherOriginName] = append(summaries[originName][otherOriginName], summary{
									Quantity:   originStake,
									InputCoin:  originName,
									OutputCoin: otherOriginName,
									Vessel:     coinName,
									Direct:     directAsk,
									Indirect:   otherToMarketBid,
									Gain:       otherToMarketBid.Add(originStake.Neg()),
								})
							}

						}
					}
				}
			}
		}
	}
}

func sortSummaries() {
	for originName := range validOrigins[exchangeName] {
		for otherOriginName := range validOrigins[exchangeName] {
			sort.Slice(summaries[originName][otherOriginName], func(aIndex, bIndex int) bool {
				a := summaries[originName][otherOriginName][aIndex]
				b := summaries[originName][otherOriginName][bIndex]
				return (b.Gain).GreaterThan(a.Gain)
			})
		}
	}
}

func orderedByGains() []string {
	output := make([]string, 0)

	for originName := range validOrigins[exchangeName] {
		for otherOriginName := range validOrigins[exchangeName] {
			if originName != otherOriginName && originName != "USDT" {
				output = append(output, originName+"-"+otherOriginName)
			}
		}
	}

	sort.Slice(output, func(aIndex, bIndex int) bool {
		aSplit := strings.Split(output[aIndex], "-")
		bSplit := strings.Split(output[bIndex], "-")
		a := summaries[aSplit[0]][aSplit[1]]
		b := summaries[bSplit[0]][bSplit[1]]
		return (b[len(b)-1].Gain).GreaterThan(a[len(a)-1].Gain)
	})
	return output
}

func printSummaries() {
	for _, marketRelationship := range orderedByGains() {
		marketRelationSplit := strings.Split(marketRelationship, "-")
		originName := marketRelationSplit[0]
		otherOriginName := marketRelationSplit[1]
		if originName != otherOriginName && originName != "USDT" {
			//				directAsk, _, _, _ := convert(marketName, otherMarketName, decimal.NewFromFloat(1))
			fmt.Printf("Origin : %v at %v \n", originName, validOrigins[exchangeName][originName])
			for _, summaryValue := range summaries[originName][otherOriginName] {
				_, _, last, _ := convert(originName, "USDT", summaryValue.Gain)
				fmt.Printf("\tIndirect %v -> %v -> %v -> %v : %v\n\t\tGain : %v\n\t\tIn USDT : %v\n", originName, summaryValue.Vessel, otherOriginName, originName, summaryValue.Indirect, summaryValue.Gain, last)
			}
		}

	}
	//for marketName := range summaries {
	//	for otherMarketName := range summaries[marketName] {
	//		if marketName != otherMarketName && marketName != "USDT" {
	//			//				directAsk, _, _, _ := convert(marketName, otherMarketName, decimal.NewFromFloat(1))
	//			fmt.Printf("Market : %v at 1.00 \n", marketName)
	//			for _, summaryValue := range summaries[marketName][otherMarketName] {
	//				_, _, last, _ := convert(otherMarketName, "USDT", summaryValue.Gain)
	//				fmt.Printf("\tIndirect %v -> %v -> %v -> %v : %v\n\t\tGain : %v\n\t\tIn USDT : %v\n", marketName, summaryValue.Vessel, otherMarketName, marketName, summaryValue.Indirect, summaryValue.Gain, last)
	//			}
	//		}
	//	}
	//}
}

/* ************************************************************************************************
 * Trading
 * ***********************************************************************************************/

func transfer(inputCoinName string, outputCoinName string) {
	_, inputIsParentCoin := parentCoins[inputCoinName]
	_, outputIsParentCoin := parentCoins[outputCoinName]

	var market string
	var quantity decimal.Decimal
	var rate decimal.Decimal
	var transferType string
	if inputIsParentCoin && outputIsParentCoin {
		switch inputCoinName {
		case "ETH":
			market = getMarketName(outputCoinName, inputCoinName)
			transferType = "sell"
			switch outputCoinName {
			case "BTC":
				rate = parentCoins[inputCoinName].Btc
			case "USDT":
				rate = parentCoins[outputCoinName].Eth
			}
		case "BTC":
			switch outputCoinName {
			case "ETH":
				market = getMarketName(inputCoinName, outputCoinName)
				transferType = "buy"
				rate = parentCoins[outputCoinName].Btc
			case "USDT":
				market = getMarketName(outputCoinName, inputCoinName)
				transferType = "sell"
				rate = parentCoins[outputCoinName].Btc
			}
		case "USDT":
			market = getMarketName(inputCoinName, outputCoinName)
			transferType = "buy"
			switch outputCoinName {
			case "ETH":
				rate = parentCoins[inputCoinName].Eth
			case "BTC":
				rate = parentCoins[inputCoinName].Btc
			}
		}
	} else if inputIsParentCoin && !outputIsParentCoin {
		market = getMarketName(inputCoinName, outputCoinName)
		transferType = "buy"
		switch inputCoinName {
		case "BTC":
			rate = childCoins[outputCoinName].Btc
		case "ETH":
			rate = childCoins[outputCoinName].Eth
		}
	} else if !inputIsParentCoin && outputIsParentCoin {
		market = getMarketName(outputCoinName, inputCoinName)
		transferType = "sell"
		switch outputCoinName {
		case "BTC":
			rate = childCoins[inputCoinName].Btc
		case "ETH":
			rate = childCoins[inputCoinName].Eth
		}
	}
	fmt.Printf("%v:\n\tmarket : %v\n\tquantity : %v\n\trate : %v\n", transferType, market, quantity, rate)
}

func transferBittrex(inputCoin string, outputCoin string) {

}

/* ****************************************************************************************
 * Display
 * ***************************************************************************************/
/*
func printCoinSummary(childCoinName string) {
	child := childCoins[childCoinName]
	_, childIsParent := parentCoins[childCoinName]
	if len(childCoins[childCoinName].ParentCoins) == 2 && !childIsParent {
		fmt.Printf("%v:\n", childCoinName)
		fmt.Printf("\tBTC : %v\n", child.Summary.ChildToBtc)
		fmt.Printf("\tETH : %v\n", child.Summary.ChildToEth)
		fmt.Println()
		fmt.Printf("\t%v -> BTC -> USD : %v\n", child.Name, child.Summary.ChildToBtc.Mul(convert("BTC", "USDT")))
		fmt.Printf("\t%v -> ETH -> USD : %v\n", child.Name, child.Summary.ChildToEth.Mul(convert("ETH", "USDT")))
		fmt.Println()

		fmt.Printf("\tBTC -> %v -> ETH: %v\n", child.Name, child.Summary.IndirectToEth)
		fmt.Printf("\tBTC -> ETH      : %v\n", child.Summary.DirectToEth)
		fmt.Printf("\tDiff ETH: %v\n", child.Summary.DiffEth)
		fmt.Printf("\tDiff in USDT: %v\n", child.Summary.DiffEthUsdt)
		fmt.Printf("\tGain per USDT: %v\n", child.Summary.DiffEthPerUsdt)

		fmt.Println()
		fmt.Printf("\tETH -> %v -> BTC  : %v\n", child.Name, child.Summary.IndirectToBtc)
		fmt.Printf("\tETH -> BTC        : %v\n", child.Summary.DirectToBtc)

		fmt.Printf("\tDiff BTC: %v\n", child.Summary.DiffBtc)

		fmt.Printf("\tDiff in USDT: %v\n", child.Summary.DiffBtcUsdt)
		fmt.Printf("\tGain per USDT: %v\n", child.Summary.DiffBtcPerUsdt)
		fmt.Println("-------------------------------------------------------------")
	}
}
*/
/* ***************************************************************************************
 * Utils
 * **************************************************************************************/

func (b *balances) printBalances() {
	b.lock.RLock()
	for bal := range b.balances {
		fmt.Println(bal + " - " + b.balances[bal].String())
	}
	b.lock.RUnlock()
}

func (b *balances) get(key string) (decimal.Decimal, bool) {
	b.lock.RLock()
	balance, exists := b.balances[key]
	b.lock.RUnlock()
	return balance, exists
}

func contains(slice []string, value string) bool {
	for _, a := range slice {
		if a == value {
			return true
		}
	}
	return false
}

func isValidRelationship(exchangeName, relationshipName string) bool {
	output := false

	switch exchangeName {
	case "Bittrex":
		_, isValid := validOrigins[exchangeName][relationshipName]
		output = isValid
	default:
		fmt.Printf("%v is not a supported exchange. Cannot validate relationship: %v\n", exchangeName, relationshipName)
	}

	return output
}

func convert(inputName string, outputName string, inputQuantity decimal.Decimal) (decimal.Decimal, decimal.Decimal, decimal.Decimal, bool) {
	outputAsk := decimal.NewFromFloat(0)
	outputBid := decimal.NewFromFloat(0)
	outputLast := decimal.NewFromFloat(0)
	outputConvertable := true
	_, coinHasRelationship := coins[inputName].Relationships[outputName]
	_, coinHasRelationshipReverse := coins[outputName].Relationships[inputName]
	if coinHasRelationship {
		withTransaction := applyTransactionFee(inputQuantity)
		outputAsk = withTransaction.Mul(coins[inputName].Relationships[outputName].Ask)
		outputBid = withTransaction.Mul(coins[inputName].Relationships[outputName].Bid)
		outputLast = withTransaction.Mul(coins[inputName].Relationships[outputName].Last)
	} else if coinHasRelationshipReverse {
		withTransaction := applyTransactionFee(inputQuantity)
		outputAsk = withTransaction.Mul(decimal.NewFromFloat(1).Div(coins[outputName].Relationships[inputName].Ask))
		outputBid = withTransaction.Mul(decimal.NewFromFloat(1).Div(coins[outputName].Relationships[inputName].Bid))
		outputLast = withTransaction.Mul(decimal.NewFromFloat(1).Div(coins[outputName].Relationships[inputName].Last))
	} else {
		//fmt.Printf("Unable to conver %v -> %v\n", inputName, outputName)
		outputConvertable = false
	}
	/*
		if inputIsParentCoin && !outputIsParentCoin {
			switch inputCoinName {
			case "BTC":
				output = applyTransactionFee(decimal.NewFromFloat(1)).Div(childCoins[outputCoinName].Btc)
			case "ETH":
				output = applyTransactionFee(decimal.NewFromFloat(1)).Div(childCoins[outputCoinName].Eth)
			}
		} else if inputIsParentCoin && outputIsParentCoin {
			switch outputCoinName {
			case "BTC":
				output = applyTransactionFee(decimal.NewFromFloat(1)).Mul(parentCoins[inputCoinName].Btc)
			case "ETH":
				output = applyTransactionFee(decimal.NewFromFloat(1)).Mul(parentCoins[inputCoinName].Eth)
			case "USDT":
				output = applyTransactionFee(decimal.NewFromFloat(1)).Mul(parentCoins[inputCoinName].Usdt)
			}
		} else if !inputIsParentCoin && outputIsParentCoin {
			switch outputCoinName {
			case "BTC":
				output = applyTransactionFee(childCoins[inputCoinName].Btc)
			case "ETH":
				output = applyTransactionFee(childCoins[inputCoinName].Eth)
			}
		}*/

	return outputAsk, outputBid, outputLast, outputConvertable
}

func getMarketName(pre, post string) string {
	return pre + "-" + post
}

func applyTransactionFee(input decimal.Decimal) decimal.Decimal {
	return input.Add((input.Mul(transactionFee).Neg()))
}

func main() {
	summaries = make(map[string]map[string][]summary)
	//var parentCoinNames = [...]string{"BTC", "ETH", "USDT"}
	bittrexThreshhold := time.Duration(10) * time.Second
	var bittrexKey, coinbaseKey, bittrexSecret string
	fmt.Printf("chaingang running\n")
	for i := 1; i < len(os.Args); i += 2 {
		switch os.Args[i] {
		case "-b":
			bittrexKey = os.Args[i+1]
		case "-c":
			coinbaseKey = os.Args[i+1]
		case "-s":
			bittrexSecret = os.Args[i+1]
		default:
			panic("unrecognized argument")
		}
	}

	fmt.Printf("\tbittrexKey: %v\n", bittrexKey)
	fmt.Printf("\tbittrexSecret: %v\n", bittrexSecret)
	fmt.Printf("\tcoinbaseKey: %v\n", coinbaseKey)

	if bittrexKey != "" && bittrexSecret != "" {
		bittrexClient := bittrex.New(bittrexKey, bittrexSecret)

		for {
			marketSummaries, err := updateMarketSummaries(bittrexClient)
			go func() {
				createCoins(marketSummaries)
				populateCoins()
				createSummaries(bittrexClient)
				sortSummaries()
				printSummaries()

				acctBalance.printBalances()
			}()
			if err == nil {

			} else {
				fmt.Println(err)
			}
			time.Sleep(bittrexThreshhold)
		}

	} else {
		fmt.Println("please provide bittrex key and secret")
	}
}
