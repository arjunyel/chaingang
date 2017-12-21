package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/toorop/go-bittrex"
)

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
	Quantity   decimal.Decimal
	InputCoin  string
	OutputCoin string
	Vessel     string
	Direct     decimal.Decimal
	Indirect   decimal.Decimal
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
	transactionFee = decimal.NewFromFloat(.0025)
	parentCoins    = map[string]*parentCoin{}
	childCoins     = map[string]*childCoin{}
	coins          = map[string]*Coin{}
	validMarkets   = map[string]map[string]bool{
		"Bittrex": map[string]bool{
			"BTC":  true,
			"ETH":  true,
			"USDT": true,
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

/* ******************************************************************
 * Populate Metrics for Child and Parent Coins
 * *****************************************************************/
func createCoins(marketSummaries []bittrex.MarketSummary) {
	for _, marketSummary := range marketSummaries {
		relationshipName := strings.Split(marketSummary.MarketName, "-")[0]
		coinName := strings.Split(marketSummary.MarketName, "-")[1]

		_, coinExists := coins[coinName]
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

	for marketName := range validMarkets[exchangeName] {
		_, marketHasCoin := coins[marketName]
		if !marketHasCoin {
			coins[marketName] = &Coin{
				Name:          marketName,
				Relationships: make(map[string]Relationship),
			}
		}
	}
}

func populateCoins() {

	for coinName, coinValue := range coins {
		for marketName := range validMarkets[exchangeName] {
			if marketName != coinName {
				_, hasRelationship := coinValue.Relationships[marketName]
				if !hasRelationship && isValidRelationship(exchangeName, coinName) {
					fmt.Printf("%v : %v\n", coinName, marketName)
					ask := decimal.NewFromFloat(0)
					bid := decimal.NewFromFloat(0)
					last := decimal.NewFromFloat(0)
					if coins[marketName].Relationships[coinName].Ask != decimal.NewFromFloat(0) {
						ask = decimal.NewFromFloat(1).Div(coins[marketName].Relationships[coinName].Ask)
					}
					if coins[marketName].Relationships[coinName].Bid != decimal.NewFromFloat(0) {
						bid = decimal.NewFromFloat(1).Div(coins[marketName].Relationships[coinName].Bid)
					}
					if coins[marketName].Relationships[coinName].Last != decimal.NewFromFloat(0) {
						last = decimal.NewFromFloat(1).Div(coins[marketName].Relationships[coinName].Last)
					}
					coinValue.Relationships[marketName] = Relationship{

						Ask:  ask,
						Bid:  bid,
						Last: last,
					}
				}
			}
		}
	}
}

func createSummaries() {
	for marketName := range validMarkets[exchangeName] {
		summaries[marketName] = make(map[string][]summary)
		for otherMarketName := range validMarkets[exchangeName] {
			if marketName != otherMarketName && marketName != "USDT" {
				summaries[marketName][otherMarketName] = make([]summary, 0)
				directAsk, _, _, _ := convert(marketName, otherMarketName, decimal.NewFromFloat(1))
				fmt.Printf("Direct Ask %v -> %v : %v\n", marketName, otherMarketName, directAsk)
				for coinName := range coins {
					_, firstBid, _, firstConvertable := convert(marketName, coinName, decimal.NewFromFloat(1))
					_, secondBid, _, secondConvertable := convert(coinName, otherMarketName, firstBid)
					if firstConvertable && secondConvertable {
						summaries[marketName][otherMarketName] = append(summaries[marketName][otherMarketName], summary{
							Quantity:   decimal.NewFromFloat(1),
							InputCoin:  marketName,
							OutputCoin: otherMarketName,
							Vessel:     coinName,
							Direct:     directAsk,
							Indirect:   secondBid,
						})
						fmt.Printf("\tIndirect %v -> %v -> %v : %v\n", marketName, coinName, otherMarketName, secondBid)
						fmt.Printf("\t\tgain in %v :  %v\n", otherMarketName, secondBid.Add(directAsk.Neg()))
					}

				}
			}
		}
	}
}

/*
func populateCoinSummary() {
	for childCoinName, childCoinValue := range childCoins {
		_, childIsParent := parentCoins[childCoinName]
		if len(childCoinValue.ParentCoins) == 2 && !childIsParent {
			childCoins[childCoinName].Summary.ChildToBtc = convert(childCoinName, "BTC")
			childCoins[childCoinName].Summary.ChildToEth = convert(childCoinName, "ETH")
			childCoins[childCoinName].Summary.IndirectToEth = convert("BTC", childCoinName).Mul(convert(childCoinName, "ETH"))
			childCoins[childCoinName].Summary.IndirectToBtc = convert("ETH", childCoinName).Mul(convert(childCoinName, "BTC"))
			childCoins[childCoinName].Summary.DirectToEth = convert("BTC", "ETH")
			childCoins[childCoinName].Summary.DirectToBtc = convert("ETH", "BTC")
			childCoins[childCoinName].Summary.DiffEth = childCoins[childCoinName].Summary.IndirectToEth.Add(childCoins[childCoinName].Summary.DirectToEth.Neg())
			childCoins[childCoinName].Summary.DiffBtc = childCoins[childCoinName].Summary.IndirectToBtc.Add(childCoins[childCoinName].Summary.DirectToBtc.Neg())
			childCoins[childCoinName].Summary.DiffEthUsdt = convert("ETH", "USDT").Mul(childCoins[childCoinName].Summary.DiffEth)
			childCoins[childCoinName].Summary.DiffBtcUsdt = convert("BTC", "USDT").Mul(childCoins[childCoinName].Summary.DiffBtc)
			childCoins[childCoinName].Summary.DiffBtcPerUsdt = childCoins[childCoinName].Summary.DiffBtcUsdt.Div(convert("BTC", "USDT"))
			childCoins[childCoinName].Summary.DiffEthPerUsdt = childCoins[childCoinName].Summary.DiffEthUsdt.Div(convert("ETH", "USDT"))
		}
	}
}*/

/* ************************************************************************************************
 * Trading
 * ***********************************************************************************************/
/*
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
*/
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
		_, isValid := validMarkets[exchangeName][relationshipName]
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

				for k, v := range coins {
					//if isValidRelationship(exchangeName, k) {
					fmt.Printf("%v\n", k)
					for n, r := range v.Relationships {
						fmt.Printf("\t%v : \n", n)
						fmt.Printf("\t\tAsk : %v\n\t\tBid : %v\n\t\tLast : %v\n", r.Ask, r.Bid, r.Last)
						askV, _, _, convertable := convert(n, k, decimal.NewFromFloat(1))
						if convertable {
							fmt.Printf("\t\t\t%v -> %v : %v\n", n, k, askV)
						}

					}
					//}

				}
				createSummaries()

				/*childCoinSlice := make([]childCoin, len(childCoins))

				childCoinSliceIndex := 0
				for _, coin := range childCoins {
					childCoinSlice[childCoinSliceIndex] = *coin
					childCoinSliceIndex++
				}
				*/
				for marketName := range validMarkets[exchangeName] {
					for otherMarketName := range validMarkets[exchangeName] {
						sort.Slice(summaries[marketName][otherMarketName], func(aIndex, bIndex int) bool {
							a := summaries[marketName][otherMarketName][aIndex]
							b := summaries[marketName][otherMarketName][bIndex]
							return (a.Direct.Add(a.Indirect.Neg())).GreaterThan(b.Indirect.Add(b.Indirect.Neg()))
						})
					}
				}
				/*

					for _, coin := range childCoinSlice {
						printCoinSummary(coin.Name)
					}
					for parentCoinName, parentCoinValue := range parentCoins {
						fmt.Printf("%v:\n", parentCoinName)
						fmt.Printf("\tBTC : %v\n", parentCoinValue.Btc)
						fmt.Printf("\tETH : %v\n", parentCoinValue.Eth)
						fmt.Printf("\tUSDT: %v\n", parentCoinValue.Usdt)

					}
					//test buy
					transfer("BTC", "ADA")
					transfer("ADA", "ETH")
				*/

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
