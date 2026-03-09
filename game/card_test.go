package game

import (
	"testing"
)

func TestLoadDeckFromYaml(t *testing.T) {
	deck, err := LoadDeckFromYaml("StandPukeTest.yaml")
	if err != nil {
		t.Fatalf("加载牌堆失败: %v", err)
	}
	println("牌堆加载成功，牌的数量:", len(deck.Cards))
	deck.Print()
}

func TestDeck_Shuffle(t *testing.T) {
	deck, err := LoadDeckFromYaml("StandPukeTest.yaml")
	if err != nil {
		t.Fatalf("加载牌堆失败: %v", err)
	}
	deck.Shuffle()
	deck.Print()
}

func TestSort(t *testing.T) {
	deck, err := LoadDeckFromYaml("poker_cards.yaml")
	if err != nil {
		t.Fatalf("加载牌堆失败: %v", err)
	}
	deck.Shuffle()
	shoupai, err := deck.Deliver(10)
	if err != nil {
		t.Fatalf("发牌失败: %v", err)
	}
	shoupai.Print()
	shoupai.Sort()
	t.Logf("排序后：")
	shoupai.Print()
}
