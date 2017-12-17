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

type marketRecord struct {
	MarketSummary bittrex.MarketSummary
}

type parentCoin struct {
	Name       string
	Btc        decimal.Decimal
	Eth        decimal.Decimal
	Usdt       decimal.Decimal
	ChildCoins []string
}

type childCoin struct {
	Name        string
	Btc         decimal.Decimal
	Eth         decimal.Decimal
	ParentCoins []string
	Summary     summary
}
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

// Global Variables
var (
	transactionFee = decimal.NewFromFloat(.0025)
)

//Storage
var parentCoins = map[string]*parentCoin{}
var childCoins = map[string]*childCoin{}

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
func populateCoins(marketSummaries []bittrex.MarketSummary) {
	for _, marketSummary := range marketSummaries {

		newParentCoinName := strings.Split(marketSummary.MarketName, "-")[0]
		newChildCoinName := strings.Split(marketSummary.MarketName, "-")[1]
		//calculate nonUSDT
		if _, childIsParent := parentCoins[newChildCoinName]; childIsParent {
			switch newParentCoinName {
			case "BTC":
				switch newChildCoinName {
				case "ETH":
					parentCoins[newChildCoinName].Btc = marketSummary.Last
					parentCoins[newParentCoinName].Eth = decimal.NewFromFloat(1).Div(marketSummary.Last)
				case "USDT":
					parentCoins[newChildCoinName].Btc = marketSummary.Last
					parentCoins[newParentCoinName].Usdt = decimal.NewFromFloat(1).Div(marketSummary.Last)
				}
			case "ETH":
				switch newChildCoinName {
				case "BTC":
					parentCoins[newChildCoinName].Eth = marketSummary.Last
					parentCoins[newParentCoinName].Btc = decimal.NewFromFloat(1).Div(marketSummary.Last)
				case "USDT":
					parentCoins[newChildCoinName].Eth = marketSummary.Last
					parentCoins[newParentCoinName].Usdt = decimal.NewFromFloat(1).Div(marketSummary.Last)
				}
			case "USDT":
				switch newChildCoinName {
				case "BTC":
					parentCoins[newChildCoinName].Usdt = marketSummary.Last
					parentCoins[newParentCoinName].Btc = decimal.NewFromFloat(1).Div(marketSummary.Last)
				case "ETH":
					parentCoins[newChildCoinName].Usdt = marketSummary.Last
					parentCoins[newParentCoinName].Eth = decimal.NewFromFloat(1).Div(marketSummary.Last)
				}
			default:
				fmt.Printf("Warning : No support for parent coin : %v\n", newParentCoinName)
			}
		}

		if _, newChildCoinExists := childCoins[newChildCoinName]; !newChildCoinExists {
			childCoins[newChildCoinName] = &childCoin{
				Name:        newChildCoinName,
				ParentCoins: make([]string, 0),
			}
		}
		switch newParentCoinName {
		case "BTC":
			childCoins[newChildCoinName].Btc = marketSummary.Last
		case "ETH":
			childCoins[newChildCoinName].Eth = marketSummary.Last
		case "USDT":
			//do nothing
		default:
			fmt.Printf("Warning : No support for parent coin : %v\n", newParentCoinName)
		}
		if newParentCoinName != "USDT" {
			if !contains(childCoins[newChildCoinName].ParentCoins, newParentCoinName) {
				childCoins[newChildCoinName].ParentCoins = append(childCoins[newChildCoinName].ParentCoins, newParentCoinName)
			}
			if !contains(parentCoins[newParentCoinName].ChildCoins, newChildCoinName) {
				parentCoins[newParentCoinName].ChildCoins = append(parentCoins[newParentCoinName].ChildCoins, newChildCoinName)
			}
		}

	}
}

func applyTransactionFee(input decimal.Decimal) decimal.Decimal {
	return input.Add((input.Mul(transactionFee).Neg()))
}

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
}

/* ************************************************************************************************
 * Trading
 * ***********************************************************************************************/

func transfer(inputCoinName string, outputCoinName string) {
	_, inputIsParentCoin := parentCoins[inputCoinName]
	_, outputIsParentCoin := parentCoins[outputCoinName]

	var market string
	var quantity int
	var rate decimal.Decimal
	var transferType string
	if inputIsParentCoin && outputIsParentCoin {
		switch inputCoinName {
		case "ETH":
			market = getMarketName(outputCoinName, inputCoinName)
			transferType = "sell"
		case "BTC":
			switch outputCoinName {
			case "ETH":
				market = getMarketName(inputCoinName, outputCoinName)
				transferType = "buy"
			case "USDT":
				market = getMarketName(outputCoinName, inputCoinName)
				transferType = "sell"
			}
		case "USDT":
			market = getMarketName(inputCoinName, outputCoinName)
			transferType = "buy"
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

/* ****************************************************************************************
 * Display
 * ***************************************************************************************/

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

func convert(inputCoinName string, outputCoinName string) decimal.Decimal {
	output := decimal.NewFromFloat(0)

	_, inputIsParentCoin := parentCoins[inputCoinName]
	_, outputIsParentCoin := parentCoins[outputCoinName]

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
			output = applyTransactionFee(parentCoins[inputCoinName].Btc)
		case "ETH":
			output = applyTransactionFee(parentCoins[inputCoinName].Eth)
		case "USDT":
			output = applyTransactionFee(parentCoins[inputCoinName].Usdt)
		}
	} else if !inputIsParentCoin && outputIsParentCoin {
		switch outputCoinName {
		case "BTC":
			output = applyTransactionFee(childCoins[inputCoinName].Btc)
		case "ETH":
			output = applyTransactionFee(childCoins[inputCoinName].Eth)
		}
	}

	return output
}

func getMarketName(pre, post string) string {
	return pre + "-" + post
}

func main() {
	var parentCoinNames = [...]string{"BTC", "ETH", "USDT"}
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

		//populateParentCoin
		for _, parentCoinName := range parentCoinNames {

			parentCoins[parentCoinName] = &parentCoin{
				Name: parentCoinName,
			}
		}

		for {
			marketSummaries, err := updateMarketSummaries(bittrexClient)
			go func() {
				populateCoins(marketSummaries)
				populateCoinSummary()

				childCoinSlice := make([]childCoin, len(childCoins))

				childCoinSliceIndex := 0
				for _, coin := range childCoins {
					childCoinSlice[childCoinSliceIndex] = *coin
					childCoinSliceIndex++
				}

				sort.Slice(childCoinSlice, func(a, b int) bool {
					var aValue, bValue float64
					childCoinA := childCoinSlice[a]
					childCoinB := childCoinSlice[b]

					if childCoinA.Summary.DiffBtcPerUsdt.GreaterThan(childCoinA.Summary.DiffEthPerUsdt) {
						aValue, _ = childCoinA.Summary.DiffBtcPerUsdt.Mul(decimal.NewFromFloat(10000)).Float64()
					} else if childCoinA.Summary.DiffBtcPerUsdt.LessThan(childCoinA.Summary.DiffEthPerUsdt) {
						aValue, _ = childCoinA.Summary.DiffEthPerUsdt.Mul(decimal.NewFromFloat(10000)).Float64()
					}

					if childCoinB.Summary.DiffBtcPerUsdt.GreaterThan(childCoinB.Summary.DiffEthPerUsdt) {
						bValue, _ = childCoinB.Summary.DiffBtcPerUsdt.Mul(decimal.NewFromFloat(10000)).Float64()
					} else if childCoinB.Summary.DiffBtcPerUsdt.LessThan(childCoinB.Summary.DiffEthPerUsdt) {
						bValue, _ = childCoinB.Summary.DiffEthPerUsdt.Mul(decimal.NewFromFloat(10000)).Float64()
					}
					return aValue < bValue
				})

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
