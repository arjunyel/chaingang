package main

import (
	"fmt"
	"os"
)

func main() {
	var bitterexKey, coinbaseKey string
	const maxArguments = 2
	fmt.Printf("chaingang running\n")
		for i := 1; i < len(os.Args); i += 2 {
			if(len(os.Args) >= i + 1){
				if(os.Args[i] == "-b"){
					bitterexKey = os.Args[i + 1]
				} else if(os.Args[i] == "-c"){
					coinbaseKey = os.Args[i + 1]
				}
			}
		}

	fmt.Printf("\tbitterexKey: %v\n", bitterexKey)
	fmt.Printf("\tcoinbaseKey: %v\n", coinbaseKey)

	if(bitterexKey != ""){
		fmt.Printf("time to call api\n")
	}
}
