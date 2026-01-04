package controllers

// VoiceOption 音色选项
type VoiceOption struct {
	Value string `json:"value"` // 音色值
	Label string `json:"label"` // 音色显示名称
}

// VoiceOptions 定义各provider的音色选项
// 根据火山引擎豆包语音文档：https://www.volcengine.com/docs/6561/97465?lang=zh
// 和豆包WebSocket文档：https://www.volcengine.com/docs/6561/1257544?lang=zh
var VoiceOptions = map[string][]VoiceOption{
	// Edge TTS 音色列表（中文）
	// 参考：https://blog.csdn.net/u012917925/article/details/134683773
	"edge": {
		{Value: "zh-CN-XiaoxiaoNeural", Label: "晓晓（女声）"},
		{Value: "zh-CN-YunxiNeural", Label: "云希（男声）"},
		{Value: "zh-CN-YunyangNeural", Label: "云扬（男声）"},
		{Value: "zh-CN-XiaoyiNeural", Label: "晓伊（女声）"},
		{Value: "zh-CN-YunjianNeural", Label: "云健（男声）"},
		{Value: "zh-CN-YunxiaNeural", Label: "云夏（男声）"},
		{Value: "zh-CN-YunhaoNeural", Label: "云皓（男声）"},
		{Value: "zh-CN-XiaohanNeural", Label: "晓涵（女声）"},
		{Value: "zh-CN-XiaomoNeural", Label: "晓墨（女声）"},
		{Value: "zh-CN-XiaoxuanNeural", Label: "晓萱（女声）"},
		{Value: "zh-CN-XiaoruiNeural", Label: "晓睿（女声）"},
		{Value: "zh-CN-XiaoshuangNeural", Label: "晓双（女声）"},
		{Value: "zh-CN-XiaoyanNeural", Label: "晓颜（女声）"},
		{Value: "zh-CN-XiaoyouNeural", Label: "晓悠（女声）"},
		{Value: "zh-CN-XiaozhenNeural", Label: "晓甄（女声）"},
		{Value: "zh-CN-YunfengNeural", Label: "云枫（男声）"},
		{Value: "zh-CN-YunyeNeural", Label: "云野（男声）"},
		{Value: "zh-CN-YunzeNeural", Label: "云泽（男声）"},
	},

	// Microsoft TTS 音色列表（中文）
	"microsoft": {
		{Value: "zh-CN-XiaoxiaoNeural", Label: "晓晓（女声）"},
		{Value: "zh-CN-YunxiNeural", Label: "云希（男声）"},
		{Value: "zh-CN-YunyangNeural", Label: "云扬（男声）"},
		{Value: "zh-CN-XiaoyiNeural", Label: "晓伊（女声）"},
		{Value: "zh-CN-YunjianNeural", Label: "云健（男声）"},
		{Value: "zh-CN-YunxiaNeural", Label: "云夏（男声）"},
		{Value: "zh-CN-YunhaoNeural", Label: "云皓（男声）"},
		{Value: "zh-CN-XiaohanNeural", Label: "晓涵（女声）"},
		{Value: "zh-CN-XiaomoNeural", Label: "晓墨（女声）"},
		{Value: "zh-CN-XiaoxuanNeural", Label: "晓萱（女声）"},
		{Value: "zh-CN-XiaoruiNeural", Label: "晓睿（女声）"},
		{Value: "zh-CN-XiaoshuangNeural", Label: "晓双（女声）"},
		{Value: "zh-CN-XiaoyanNeural", Label: "晓颜（女声）"},
		{Value: "zh-CN-XiaoyouNeural", Label: "晓悠（女声）"},
		{Value: "zh-CN-XiaozhenNeural", Label: "晓甄（女声）"},
		{Value: "zh-CN-YunfengNeural", Label: "云枫（男声）"},
		{Value: "zh-CN-YunyeNeural", Label: "云野（男声）"},
		{Value: "zh-CN-YunzeNeural", Label: "云泽（男声）"},
	},

	// 豆包 TTS 音色列表（HTTP接口）
	// 参考：https://www.volcengine.com/docs/6561/97465?lang=zh
	"doubao": {
		{Value: "BV700_V2_streaming", Label: "灿灿 2.0"},
		{Value: "BV705_streaming", Label: "炀炀"},
		{Value: "BV701_V2_streaming", Label: "擎苍 2.0"},
		{Value: "BV001_V2_streaming", Label: "通用女声 2.0"},
		{Value: "BV700_streaming", Label: "灿灿"},
		{Value: "BV406_V2_streaming", Label: "超自然音色-梓梓2.0"},
		{Value: "BV406_streaming", Label: "超自然音色-梓梓"},
		{Value: "BV407_V2_streaming", Label: "超自然音色-燃燃2.0"},
		{Value: "BV407_streaming", Label: "超自然音色-燃燃"},
		{Value: "BV001_streaming", Label: "通用女声"},
		{Value: "BV002_streaming", Label: "通用男声"},
		{Value: "BV701_streaming", Label: "擎苍"},
		{Value: "BV119_streaming", Label: "通用赘婿"},
		{Value: "BV102_streaming", Label: "儒雅青年"},
		{Value: "BV113_streaming", Label: "甜宠少御"},
		{Value: "BV115_streaming", Label: "古风少御"},
		{Value: "BV007_streaming", Label: "亲切女声"},
		{Value: "BV056_streaming", Label: "阳光男声"},
		{Value: "BV005_streaming", Label: "活泼女声"},
		{Value: "BV051_streaming", Label: "奶气萌娃"},
		{Value: "BV034_streaming", Label: "知性姐姐-双语"},
		{Value: "BV033_streaming", Label: "温柔小哥"},
		{Value: "BV021_streaming", Label: "东北老铁"},
		{Value: "BV019_streaming", Label: "重庆小伙"},
		{Value: "BV213_streaming", Label: "广西表哥"},
		{Value: "BV503_streaming", Label: "活力女声-Ariana"},
		{Value: "BV504_streaming", Label: "活力男声-Jackson"},
		{Value: "BV522_streaming", Label: "气质女生"},
		{Value: "BV524_streaming", Label: "日语男声"},
		{Value: "BV104_streaming", Label: "温柔淑女"},
		{Value: "BV004_streaming", Label: "开朗青年"},
		{Value: "BV009_streaming", Label: "知性女声"},
		{Value: "BV008_streaming", Label: "亲切男声"},
		{Value: "BV064_streaming", Label: "小萝莉"},
		{Value: "BV437_streaming", Label: "解说小帅-多情感"},
		{Value: "BV511_streaming", Label: "慵懒女声-Ava"},
		{Value: "BV040_streaming", Label: "亲切女声-Anna"},
		{Value: "BV138_streaming", Label: "情感女声-Lawrence"},
		{Value: "BV704_streaming", Label: "方言灿灿"},
		{Value: "BV702_streaming", Label: "Stefan"},
		{Value: "BV421_streaming", Label: "天才少女"},
	},

	// 豆包 WebSocket TTS 音色列表
	// 参考：https://www.volcengine.com/docs/6561/1257544?lang=zh
	// 注意：doubao_ws使用的音色格式为 zh_female_xxx_bigtts 或 zh_male_xxx_bigtts 格式
	// 根据文档，音色名称格式为：zh_{gender}_{name}_bigtts
	// 由于文档页面需要JavaScript才能查看完整内容，这里列出常见的音色
	// 用户也可以手动输入不在列表中的音色值

	"doubao_ws": {
		// 女声音色
		{Value: "zh_female_wanwanxiaohe_moon_bigtts", Label: "湾湾小何（女声）"},
		{Value: "zh_female_qinqienvsheng_moon_bigtts", Label: "亲切女声（女声）"},
		{Value: "zh_female_vv_mars_bigtts", Label: "Vivi（女声）"},
		{Value: "zh_female_tianmeixiaoyuan_moon_bigtts", Label: "甜美小源（女声）"},
		{Value: "zh_female_qingchezizi_moon_bigtts", Label: "清澈梓梓（女声）"},
		{Value: "zh_female_kailangjiejie_moon_bigtts", Label: "开朗姐姐（女声）"},
		{Value: "zh_female_tianmeiyueyue_moon_bigtts", Label: "甜美悦悦（女声）"},
		{Value: "zh_female_xinlingjitang_moon_bigtts", Label: "心灵鸡汤（女声）"},
		{Value: "zh_female_linjianvhai_moon_bigtts", Label: "邻家女孩（女声）"},
		{Value: "zh_female_shuangkuaisisi_moon_bigtts", Label: "爽快思思/Skye（女声）"},
		{Value: "zh_female_gaolengyujie_moon_bigtts", Label: "高冷御姐（女声）"},
		{Value: "zh_female_meilinvyou_moon_bigtts", Label: "魅力女友（女声）"},
		{Value: "zh_female_sajiaonvyou_moon_bigtts", Label: "柔美女友（撒娇）（女声）"},
		{Value: "zh_female_yuanqinvyou_moon_bigtts", Label: "撒娇学妹（女声）"},
		{Value: "ICL_zh_female_wenrounvshen_239eff5e8ffa_tob", Label: "温柔女神（女声）"},
		{Value: "ICL_zh_female_chunzhenshaonv_e588402fb8ad_tob", Label: "纯真少女（女声）"},
		{Value: "ICL_zh_female_jinglingxiangdao_1beb294a9e3e_tob", Label: "精灵向导（女声）"},
		{Value: "ICL_zh_female_yilin_tob", Label: "贴心妹妹（女声）"},
		{Value: "ICL_zh_female_chengshujiejie_tob", Label: "成熟姐姐（女声）"},
		{Value: "ICL_zh_female_bingjiaojiejie_tob", Label: "病娇姐姐（女声）"},
		{Value: "ICL_zh_female_wumeiyujie_tob", Label: "妩媚御姐（女声）"},
		{Value: "ICL_zh_female_aojiaonvyou_tob", Label: "傲娇女友（女声）"},
		{Value: "ICL_zh_female_tiexinnvyou_tob", Label: "贴心女友（女声）"},
		{Value: "ICL_zh_female_xingganyujie_tob", Label: "性感御姐（女声）"},
		{Value: "ICL_zh_female_lixingyuanzi_cs_tob", Label: "理性圆子（客服女声）"},

		// 男声音色
		{Value: "zh_male_yangguangqingnian_moon_bigtts", Label: "阳光青年（男声）"},
		{Value: "zh_male_qingshuangnanda_mars_bigtts", Label: "清爽男大（男声）"},
		{Value: "zh_male_jieshuoxiaoming_moon_bigtts", Label: "解说小明（男声）"},
		{Value: "zh_male_linjiananhai_moon_bigtts", Label: "邻家男孩（男声）"},
		{Value: "zh_male_yuanboxiaoshu_moon_bigtts", Label: "渊博小叔（男声）"},
		{Value: "zh_male_wennuanahu_moon_bigtts", Label: "温暖阿虎/Alvin（男声）"},
		{Value: "zh_male_shaonianzixin_moon_bigtts", Label: "少年梓辛/Brayan（男声）"},
		{Value: "zh_male_beijingxiaoye_moon_bigtts", Label: "北京小爷（男声）"},
		{Value: "zh_male_jingqiangkanye_moon_bigtts", Label: "京腔侃爷/Harmony（男声）"},
		{Value: "zh_male_guozhoudege_moon_bigtts", Label: "广州德哥（男声）"},
		{Value: "zh_male_haoyuxiaoge_moon_bigtts", Label: "浩宇小哥（男声）"},
		{Value: "zh_male_shenyeboke_moon_bigtts", Label: "深夜播客（男声）"},
		{Value: "zh_male_aojiaobazong_moon_bigtts", Label: "傲娇霸总（男声）"},
		{Value: "zh_male_dongfanghaoran_moon_bigtts", Label: "东方浩然（男声）"},
		{Value: "zh_male_M100_conversation_wvae_bigtts", Label: "悠悠君子/Lucas（男声）"},
		{Value: "zh_male_xudong_conversation_wvae_bigtts", Label: "快乐小东/Daníel（男声）"},
		{Value: "zh_male_qingyiyuxuan_mars_bigtts", Label: "阳光阿辰（男声）"},
		{Value: "en_male_jason_conversation_wvae_bigtts", Label: "开朗学长（男声）"},
		{Value: "ICL_zh_male_lengkugege_v1_tob", Label: "冷酷哥哥（男声）"},
		{Value: "ICL_zh_male_shenmi_v1_tob", Label: "机灵小伙（男声）"},
		{Value: "ICL_zh_male_BV705_streaming_cs_tob", Label: "炀炀（男声）"},
		{Value: "ICL_zh_male_menyoupingxiaoge_ffed9fc2fee7_tob", Label: "闷油瓶小哥（男声）"},
		{Value: "ICL_zh_male_anrenqinzhu_cd62e63dcdab_tob", Label: "黯刃秦主（男声）"},
		{Value: "ICL_zh_male_guaogongzi_v1_tob", Label: "孤傲公子（男声）"},
		{Value: "ICL_zh_male_bingruogongzi_tob", Label: "病弱公子（男声）"},
		{Value: "ICL_zh_male_bingjiaodidi_tob", Label: "病娇弟弟（男声）"},
		{Value: "ICL_zh_male_aomanshaoye_tob", Label: "傲慢少爷（男声）"},
		{Value: "ICL_zh_male_chunzhenxuedi_tob", Label: "纯真学弟（男声）"},
		{Value: "ICL_zh_male_yourougongzi_tob", Label: "优柔公子（男声）"},
		{Value: "ICL_zh_male_tiexinnanyou_tob", Label: "贴心男友（男声）"},
		{Value: "ICL_zh_male_shaonianjiangjun_tob", Label: "少年将军（男声）"},
		{Value: "ICL_zh_male_bingjiaogege_tob", Label: "病娇哥哥（男声）"},
		{Value: "ICL_zh_male_xuebanantongzhuo_tob", Label: "学霸男同桌（男声）"},
		{Value: "ICL_zh_male_youmoshushu_tob", Label: "幽默叔叔（男声）"},
		{Value: "ICL_zh_male_wenrounantongzhuo_tob", Label: "温柔男同桌（男声）"},
		{Value: "ICL_zh_male_youmodaye_tob", Label: "幽默大爷（男声）"},
		{Value: "ICL_zh_male_shenmifashi_tob", Label: "神秘法师（男声）"},
		{Value: "ICL_zh_male_lengjunshangsi_tob", Label: "冷峻上司（男声）"},
		{Value: "ICL_en_male_michael_tob", Label: "Michael（美式英语男声）"},

		// IP/特色音色
		{Value: "zh_male_lubanqihao_mars_bigtts", Label: "鲁班七号（男声）"},
		{Value: "zh_female_yangmi_mars_bigtts", Label: "林潇（女声）"},
		{Value: "zh_female_linzhiling_mars_bigtts", Label: "玲玲姐姐（女声）"},
		{Value: "zh_female_jiyejizi2_mars_bigtts", Label: "春日部姐姐（女声）"},
		{Value: "zh_male_tangseng_mars_bigtts", Label: "唐僧（男声）"},
		{Value: "zh_male_zhubajie_mars_bigtts", Label: "猪八戒（男声）"},
		{Value: "zh_female_naying_mars_bigtts", Label: "直率英子（女声）"},
		{Value: "zh_female_leidian_mars_bigtts", Label: "女雷神（女声）"},
		{Value: "zh_male_sunwukong_mars_bigtts", Label: "猴哥（男声）"},
		{Value: "zh_male_xionger_mars_bigtts", Label: "熊二（男声）"},
		{Value: "zh_female_peiqi_mars_bigtts", Label: "佩奇猪（女声）"},
		{Value: "zh_female_yingtaowanzi_mars_bigtts", Label: "樱桃丸子（女声）"},
		{Value: "zh_male_silang_mars_bigtts", Label: "四郎（男声）"},
	},
}

// GetVoiceOptionsByProvider 根据provider获取音色列表
func GetVoiceOptionsByProvider(provider string) []VoiceOption {
	if voices, ok := VoiceOptions[provider]; ok {
		return voices
	}
	return []VoiceOption{}
}
