package mint

import (
	"fmt"
	"math/big"
	"reflect"
	"strconv"
	"testing"
)

func TestCalcBlockSubsidy(t *testing.T) {

	type test struct {
		name   string
		height int64
		want   *big.Int
	}
	tests := make([]test, 8)
	tests[0] = test{strconv.Itoa(0), 0, baseSubsidy}
	tests[1] = test{strconv.Itoa(1), 1, baseSubsidy}
	tests[2] = test{
		fmt.Sprintf("%v", SubsidyReductionInterval-1),
		SubsidyReductionInterval - 1,
		baseSubsidy}
	tests[3] = test{
		fmt.Sprintf("%v", SubsidyReductionInterval),
		SubsidyReductionInterval,
		new(big.Int).Quo(baseSubsidy, big.NewInt(2))}
	tests[4] = test{
		fmt.Sprintf("%v", SubsidyReductionInterval+1),
		SubsidyReductionInterval + 1,
		new(big.Int).Quo(baseSubsidy, big.NewInt(2))}
	tests[5] = test{
		fmt.Sprintf("%v", SubsidyReductionInterval*2-1),
		SubsidyReductionInterval*2 - 1,
		new(big.Int).Quo(baseSubsidy, big.NewInt(2))}
	tests[6] = test{
		fmt.Sprintf("%v", SubsidyReductionInterval*2),
		SubsidyReductionInterval * 2,
		new(big.Int).Quo(baseSubsidy, big.NewInt(4))}
	tests[7] = test{
		fmt.Sprintf("%v", SubsidyReductionInterval*2+1),
		SubsidyReductionInterval*2 + 1,
		new(big.Int).Quo(baseSubsidy, big.NewInt(4))}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalcBlockSubsidy(tt.height); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CalcBlockSubsidy() = %v, want %v", got, tt.want)
			}
		})
	}
}
