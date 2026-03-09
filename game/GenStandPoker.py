import yaml

def generate_poker_deck():
    # 定义花色和点数
    suits = ["红桃", "黑桃", "方块", "梅花"]
    values = ["A", "2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K"]
    
    cards = []

    # 生成普通牌 (52张)
    for suit in suits:
        for value in values:
            name = f"{suit}{value}"
            card = {
                "suit": suit,
                "value": value,
                "name": name,
                "skill": "无技能",
                "rarity": "普通"
            }
            cards.append(card)

    # 生成王牌 (2张)
    # 王牌通常没有花色，或者花色定义为"王牌"，这里为了格式统一，suit设为"无"，value设为"王"
    jokers = [
        {
            "suit": "无",
            "value": "小王",
            "name": "小王",
            "skill": "无技能",
            "rarity": "特殊"
        },
        {
            "suit": "无",
            "value": "大王",
            "name": "大王",
            "skill": "无技能",
            "rarity": "特殊"
        }
    ]
    
    cards.extend(jokers)

    # 构建最终的数据结构
    output_data = {"cards": cards}

    # 写入 YAML 文件
    # allow_unicode=True 确保中文字符正常显示而不是转义
    # sort_keys=False 保持字典插入顺序（虽然YAML标准不强制，但为了美观）
    with open("poker_cards.yaml", "w", encoding="utf-8") as f:
        yaml.dump(output_data, f, allow_unicode=True, sort_keys=False, default_flow_style=False)

    print(f"成功生成扑克牌文件：poker_cards.yaml，共 {len(cards)} 张牌。")

if __name__ == "__main__":
    # 需要安装 pyyaml 库: pip install pyyaml
    try:
        generate_poker_deck()
    except ImportError:
        print("错误: 未找到 'yaml' 模块。请先运行 'pip install pyyaml' 安装它。")