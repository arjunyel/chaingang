package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"github.com/toorop/go-bittrex"
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
	bittrexClient  *bittrex.Bittrex
	live           = *flag.Bool("l", false, "Live")
	transactionFee = decimal.NewFromFloat(.0025)
	parentCoins    = map[string]*parentCoin{}
	childCoins     = map[string]*childCoin{}
	coins          = map[string]*Coin{}
	validOrigins   = map[string]map[string]decimal.Decimal{
		"Bittrex": {
			"BTC":  decimal.NewFromFloat(0.0072),
			"ETH":  decimal.NewFromFloat(0.072),
			"USDT": decimal.NewFromFloat(100),
		},
	}
	validMarkets = map[string]map[string]bool{
		"Bittrex": {
			"BTC-ETH":  true,
			"USDT-BTC": true,
			"USDT-ETH": true,
		},
	}
	exchangeName = "Bittrex"
	summaries    map[string]map[string][]summary
	details      = *flag.Bool("details", false, "Details")
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
		_, _, aLast, _ := convert(aSplit[0], "USDT", a[len(a)-1].Gain)
		_, _, bLast, _ := convert(bSplit[0], "USDT", b[len(b)-1].Gain)
		return (bLast).GreaterThan(aLast)
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
}

func makeBestTrade(offset int) {
	ordered := orderedByGains()
	bestMarketRelationship := ordered[len(ordered)-(1+offset)]
	marketRelationSplit := strings.Split(bestMarketRelationship, "-")
	originName := marketRelationSplit[0]
	otherOriginName := marketRelationSplit[1]
	if originName != otherOriginName {
		summaryValue := summaries[originName][otherOriginName][len(summaries[originName][otherOriginName])-1]
		if live {
			executeIndirectRoute(originName, summaryValue.Vessel, otherOriginName)
		}
	}
}

func executeIndirectRoute(origin string, vessel string, outputOrigin string) {
	if live {
		fmt.Printf("Do live trade\n")
		round1 := transfer(origin, vessel, validOrigins[exchangeName][origin])
		fmt.Printf("end : %v\n", round1)
		round2 := transfer(vessel, outputOrigin, round1)
		fmt.Printf("end : %v\n", round2)
		round3 := transfer(outputOrigin, origin, round2)
		fmt.Printf("end : %v\n", round3)
	}
}

/* ************************************************************************************************
 * Trading
 * ***********************************************************************************************/

func transfer(inputCoinName string, outputCoinName string, quantity decimal.Decimal) decimal.Decimal {
	var limitType string
	var market string
	var rate decimal.Decimal
	var output decimal.Decimal
	relationship, relationshipExists := coins[outputCoinName].Relationships[inputCoinName]

	if relationshipExists {
		market = inputCoinName + "-" + outputCoinName
		limitType = "buy"
		rate = relationship.Bid
	} else {
		market = outputCoinName + "-" + inputCoinName
		limitType = "sell"
		relationship := coins[inputCoinName].Relationships[outputCoinName]
		rate = decimal.NewFromFloat(1).Div(relationship.Ask)
	}
	output = quantity.Div(rate)
	fmt.Printf("%v : \n\tin: %v \n\tout: %v \n\ttype: %v \n\tquantity: %v \n\trate: %v\n", market, inputCoinName, outputCoinName, limitType, quantity, rate)
	return output
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
	outputConvertible := true
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
		outputConvertible = false
	}
	return outputAsk, outputBid, outputLast, outputConvertible
}

func getMarketName(pre, post string) string {
	return pre + "-" + post
}

func applyTransactionFee(input decimal.Decimal) decimal.Decimal {
	return input.Add(input.Mul(transactionFee).Neg())
}

func main() {
	summaries = make(map[string]map[string][]summary)
	bittrexThreshold := time.Duration(10) * time.Second
	fmt.Printf("chaingang running\n")

	bittrexKey := os.Getenv("BITTREXKEY")
	bittrexSecret := os.Getenv("BITTREXSECRET")

	flag.Parse()

	fmt.Printf("\tbittrexKey: %v\n", bittrexKey)
	fmt.Printf("\tbittrexSecret: %v\n", bittrexSecret)

	if bittrexKey != "" && bittrexSecret != "" {
		bittrexClient = bittrex.New(bittrexKey, bittrexSecret)

		for {
			marketSummaries, err := updateMarketSummaries(bittrexClient)
			go func() {
				createCoins(marketSummaries)
				populateCoins()
				createSummaries(bittrexClient)
				sortSummaries()
				printSummaries()
				makeBestTrade(0)

				acctBalance.printBalances()
			}()
			if err == nil {

			} else {
				fmt.Println(err)
			}
			time.Sleep(bittrexThreshold)
		}
	} else {
		fmt.Println("please provide bittrex key and secret")
	}
}
