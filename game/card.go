package game

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"time"

	"gopkg.in/yaml.v2"
)

type Skill struct {
	Name        string //技能名称
	Description string //技能描述
}

type Card struct {
	Suit     string  `yaml:"suit"`      // 花色
	Value    string  `yaml:"value"`     // 牌面值
	Name     string  `yaml:"name"`      // 牌名称
	Rarity   string  `yaml:"rarity"`    // 稀有度
	ImageURL string  `yaml:"image_url"` // 图片URL
	Desc     string  `yaml:"desc"`      // 描述
	Skill    []Skill `yaml:"skills"`    // 技能列表
}

type Deck struct {
	Cards []Card //牌堆中的牌
}

func LoadDeckFromYaml(filename string) (*Deck, error) {
	// 读取文件内容
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	var deck Deck
	// 解析 YAML 内容到结构体
	err = yaml.Unmarshal(data, &deck)
	if err != nil {
		return nil, fmt.Errorf("解析 YAML 失败: %w", err)
	}

	return &deck, nil
}

// 打印牌堆中的牌
func (d *Deck) Print() {
	for _, card := range d.Cards {
		fmt.Println(card.Suit)
		fmt.Println(card.Value)
		fmt.Println(card.Name)
		fmt.Println(card.Rarity)
		fmt.Println(card.ImageURL)
		fmt.Println(card.Desc)
		for _, skill := range card.Skill {
			fmt.Println("技能名称:", skill.Name)
			fmt.Println("技能描述:", skill.Description)
		}
		fmt.Println("-------------")
	}
}

// 洗牌方法
func (d *Deck) Shuffle() {
	rand.Seed(time.Now().UnixNano()) // 使用当前时间作为随机数种子
	rand.Shuffle(len(d.Cards), func(i, j int) {
		d.Cards[i], d.Cards[j] = d.Cards[j], d.Cards[i] // 交换位置
	})
}

// 从牌堆顶部抽取一张牌
func (d *Deck) Draw() *Card {
	if len(d.Cards) == 0 {
		return nil //牌堆为空，无法抽牌
	}
	card := d.Cards[0]    //抽取牌堆顶部的牌
	d.Cards = d.Cards[1:] //更新牌堆，移除抽取的牌
	return &card
}

// 抽牌函数，从牌堆中抽取指定数量的牌
func (d *Deck) Deliver(num int) (*Deck, error) {
	if num <= 0 {
		return nil, fmt.Errorf("抽牌数量必须大于0")
	}
	if num > len(d.Cards) {
		return nil, fmt.Errorf("牌堆中没有足够的牌")
	}

	// 创建一个新的牌堆
	newDeck := &Deck{}
	for i := 0; i < num; i++ {
		card := d.Draw()
		newDeck.Cards = append(newDeck.Cards, *card)
	}

	return newDeck, nil
}

// 将指定数量的牌放回牌堆顶部
func (d *Deck) ReturnCards(cards []Card) {
	d.Cards = append(cards, d.Cards...) //将新牌放在原牌堆前面
	cards = nil                         //清空传入的牌切片，帮助 GC 回收内存
}

// 面值和花色的权重
var valueWeight = map[string]int{
	"3":  30,
	"4":  40,
	"5":  50,
	"6":  60,
	"7":  70,
	"8":  80,
	"9":  90,
	"10": 100,
	"J":  110,
	"Q":  120,
	"K":  130,
	"A":  140,
	"2":  150,
	"小王": 160,
	"大王": 170,
	"红桃": 4,
	"黑桃": 3,
	"梅花": 2,
	"方块": 1,
}

// 给手牌排序
func (d *Deck) Sort() {
	if len(d.Cards) == 0 || len(d.Cards) == 1 {
		return
	}
	//冒泡排序
	for i := 0; i < len(d.Cards)-1; i++ {
		for j := 0; j < len(d.Cards)-i-1; j++ {
			if valueWeight[d.Cards[j].Value]+valueWeight[d.Cards[j].Suit] < valueWeight[d.Cards[j+1].Value]+valueWeight[d.Cards[j+1].Suit] {
				d.Cards[j], d.Cards[j+1] = d.Cards[j+1], d.Cards[j]
			}
		}
	}
}
