package app

import "strings"

const (
	wechatApplymentDocURL  = "https://pay.wechatpay.cn/doc/v3/partner/4012719997"
	wechatMicroDocURL      = "https://pay.wechatpay.cn/doc/v3/partner/4012722249"
	wechatSettlementDocURL = "https://pay.wechatpay.cn/doc/v2/partner/4014115425"
)

type onboardingFieldMapping struct {
	LocalField  string `json:"localField"`
	Label       string `json:"label"`
	WechatField string `json:"wechatField"`
	WechatPath  string `json:"wechatPath"`
	Sensitive   bool   `json:"sensitive"`
	Note        string `json:"note,omitempty"`
}

type onboardingMediaRequirement struct {
	FieldName   string `json:"fieldName"`
	Label       string `json:"label"`
	WechatField string `json:"wechatField"`
	WechatPath  string `json:"wechatPath"`
	Required    bool   `json:"required"`
	Requirement string `json:"requirement"`
}

type onboardingIndustryRequirement struct {
	SubjectType       string `json:"subjectType"`
	SettlementID      string `json:"settlementId"`
	Industry          string `json:"industry"`
	QualificationMode string `json:"qualificationMode"`
	Requirement       string `json:"requirement"`
	SourceURL         string `json:"sourceUrl"`
}

var commonWechatOnboardingFieldMappings = []onboardingFieldMapping{
	{LocalField: "application.subjectType", Label: "主体类型", WechatField: "subject_type", WechatPath: "subject_info.subject_type"},
	{LocalField: "application.businessScene", Label: "经营场景", WechatField: "sales_scenes_type / micro_biz_type", WechatPath: "business_info.sales_info.sales_scenes_type / subject_info.micro_biz_info.micro_biz_type"},
	{LocalField: "application.merchantShortName", Label: "商户简称", WechatField: "merchant_shortname", WechatPath: "business_info.merchant_shortname"},
	{LocalField: "application.servicePhone", Label: "客服电话", WechatField: "service_phone", WechatPath: "business_info.service_phone"},
	{LocalField: "application.businessAddress", Label: "实际经营地址", WechatField: "biz_store_address / micro_address", WechatPath: "business_info.sales_info.biz_store_info.biz_store_address / subject_info.micro_biz_info"},
	{LocalField: "application.operatorName", Label: "超级管理员姓名", WechatField: "contact_name", WechatPath: "contact_info.contact_name", Sensitive: true},
	{LocalField: "application.contactPhone", Label: "超级管理员手机号", WechatField: "mobile_phone", WechatPath: "contact_info.mobile_phone", Sensitive: true},
	{LocalField: "application.contactEmail", Label: "超级管理员邮箱", WechatField: "contact_email", WechatPath: "contact_info.contact_email", Sensitive: true},
	{LocalField: "application.licenseNumber", Label: "注册号／统一社会信用代码", WechatField: "license_number", WechatPath: "subject_info.business_license_info.license_number"},
	{LocalField: "sensitive.idCardName", Label: "身份证姓名", WechatField: "id_card_name", WechatPath: "subject_info.identity_info.id_card_info.id_card_name", Sensitive: true},
	{LocalField: "sensitive.idCardNumber", Label: "身份证号码", WechatField: "id_card_number", WechatPath: "subject_info.identity_info.id_card_info.id_card_number", Sensitive: true},
	{LocalField: "sensitive.idCardAddress", Label: "身份证住址", WechatField: "id_card_address", WechatPath: "subject_info.identity_info.id_card_info.id_card_address", Sensitive: true},
	{LocalField: "sensitive.cardPeriodBegin", Label: "身份证有效期开始", WechatField: "card_period_begin", WechatPath: "subject_info.identity_info.id_card_info.card_period_begin"},
	{LocalField: "sensitive.cardPeriodEnd", Label: "身份证有效期结束", WechatField: "card_period_end", WechatPath: "subject_info.identity_info.id_card_info.card_period_end"},
	{LocalField: "sensitive.merchantName", Label: "营业执照主体全称", WechatField: "merchant_name", WechatPath: "subject_info.business_license_info.merchant_name"},
	{LocalField: "sensitive.legalPerson", Label: "经营者／法定代表人", WechatField: "legal_person", WechatPath: "subject_info.business_license_info.legal_person"},
	{LocalField: "sensitive.accountType", Label: "结算账户类型", WechatField: "bank_account_type", WechatPath: "bank_account_info.bank_account_type"},
	{LocalField: "sensitive.accountName", Label: "结算账户名称", WechatField: "account_name", WechatPath: "bank_account_info.account_name", Sensitive: true},
	{LocalField: "sensitive.accountNumber", Label: "结算银行账号", WechatField: "account_number", WechatPath: "bank_account_info.account_number", Sensitive: true},
	{LocalField: "sensitive.accountBank", Label: "开户银行", WechatField: "account_bank", WechatPath: "bank_account_info.account_bank"},
	{LocalField: "sensitive.bankAddressCode", Label: "开户银行省市编码", WechatField: "bank_address_code", WechatPath: "bank_account_info.bank_address_code"},
	{LocalField: "sensitive.bankBranchId", Label: "联行号", WechatField: "bank_branch_id", WechatPath: "bank_account_info.bank_branch_id"},
	{LocalField: "sensitive.bankName", Label: "开户支行全称", WechatField: "bank_name", WechatPath: "bank_account_info.bank_name"},
	{LocalField: "sensitive.storeName", Label: "门店／经营名称", WechatField: "biz_store_name / micro_name / micro_mobile_name", WechatPath: "business_info.sales_info.biz_store_info / subject_info.micro_biz_info"},
	{LocalField: "sensitive.storeAddressCode", Label: "门店省市编码", WechatField: "biz_address_code / micro_address_code / micro_mobile_city", WechatPath: "business_info.sales_info.biz_store_info / subject_info.micro_biz_info"},
	{LocalField: "sensitive.settlementId", Label: "结算规则 ID", WechatField: "settlement_id", WechatPath: "settlement_info.settlement_id"},
	{LocalField: "sensitive.qualificationType", Label: "所属行业", WechatField: "qualification_type", WechatPath: "settlement_info.qualification_type"},
}

var registeredIndustryRules = map[string][]onboardingIndustryRequirement{
	"INDIVIDUAL": {
		{Industry: "餐饮", QualificationMode: "ALTERNATIVE", Requirement: "以下方式二选一：食品经营许可证／餐饮服务许可证等；或门头、店内环境、收银台三类经营照片。"},
		{Industry: "食品生鲜", QualificationMode: "CONDITIONAL", Requirement: "销售食用初级农产品无需特殊资质；销售加工食品需食品流通、食品卫生、食品经营或保健食品经营卫生许可证之一。使用非本主体证照还需食品供销协议。"},
		{Industry: "私立/民营医院/诊所", QualificationMode: "REQUIRED", Requirement: "必须提供《医疗机构执业许可证》。"},
		{Industry: "保健器械/医疗器械/非处方药品", QualificationMode: "REQUIRED", Requirement: "互联网售药需《互联网药品交易服务证》；线下卖药需《药品经营许可证》；医疗器械需《医疗器械经营企业许可证》。"},
		{Industry: "游艺厅/KTV/网吧", QualificationMode: "REQUIRED", Requirement: "游艺厅/KTV 需《娱乐场所许可证》；网吧需《网络文化经营许可证》。"},
		{Industry: "机票/机票代理", QualificationMode: "REQUIRED", Requirement: "必须提供《航空公司营业执照》或《航空公司机票代理资格证》。"},
		{Industry: "宠物医院", QualificationMode: "REQUIRED", Requirement: "必须提供《动物诊疗许可证》。"},
		{Industry: "教育/培训/考试缴费/学费", QualificationMode: "REQUIRED", Requirement: "必须提供《办学许可证》。"},
		{Industry: "零售批发/生活娱乐/其他", QualificationMode: "NONE", Requirement: "微信结算规则表未要求行业特殊资质，不得提交 settlement_info.qualifications。"},
	},
	"ENTERPRISE": {
		{Industry: "餐饮", QualificationMode: "ALTERNATIVE", Requirement: "以下方式二选一：食品经营许可证／餐饮服务许可证等；或门头、店内环境、收银台三类经营照片。"},
		{Industry: "食品生鲜", QualificationMode: "CONDITIONAL", Requirement: "销售食用初级农产品无需特殊资质；销售加工食品需食品流通、食品卫生、食品经营或保健食品经营卫生许可证之一。使用非本主体证照还需食品供销协议。"},
		{Industry: "电信运营商/宽带收费", QualificationMode: "REQUIRED", Requirement: "必须提供《电信业务经营许可证》。"},
		{Industry: "私立/民营医院/诊所", QualificationMode: "REQUIRED", Requirement: "必须提供《医疗机构执业许可证》。"},
		{Industry: "保健器械/医疗器械/非处方药品", QualificationMode: "REQUIRED", Requirement: "互联网售药需《互联网药品交易服务证》；线下卖药需《药品经营许可证》；医疗器械需《医疗器械经营企业许可证》。"},
		{Industry: "游艺厅/KTV/网吧", QualificationMode: "REQUIRED", Requirement: "游艺厅/KTV 需《娱乐场所许可证》；网吧需《网络文化经营许可证》。"},
		{Industry: "机票/机票代理", QualificationMode: "REQUIRED", Requirement: "必须提供《航空公司营业执照》或《航空公司机票代理资格证》。"},
		{Industry: "宠物医院", QualificationMode: "REQUIRED", Requirement: "必须提供《动物诊疗许可证》。"},
		{Industry: "旅行社", QualificationMode: "REQUIRED", Requirement: "必须提供《旅行社业务经营许可证》。"},
		{Industry: "宗教组织", QualificationMode: "REQUIRED", Requirement: "必须提供《宗教活动场所登记证》。"},
		{Industry: "房地产/房产中介", QualificationMode: "CONDITIONAL", Requirement: "房地产开发商需建设用地规划、建设工程规划、建筑工程开工、国有土地使用、商品房预售五项许可证；房地产中介无需特殊资质。"},
		{Industry: "共享服务", QualificationMode: "REQUIRED", Requirement: "必须提供与商业银行签订且在有效期内的资金监管协议，结算账户须与资金监管账户一致。"},
		{Industry: "文物经营/文物复制品销售", QualificationMode: "REQUIRED", Requirement: "销售文物必须提供《文物经营许可证》。"},
		{Industry: "拍卖典当", QualificationMode: "REQUIRED", Requirement: "拍卖需《拍卖经营批准证书》；典当需《典当经营许可证》。"},
		{Industry: "教育/培训/考试缴费/学费", QualificationMode: "REQUIRED", Requirement: "必须提供《办学许可证》。"},
		{Industry: "零售批发/生活娱乐/网上商城/其他", QualificationMode: "NONE", Requirement: "微信结算规则表未要求行业特殊资质，不得提交 settlement_info.qualifications。"},
	},
}

func expectedWechatSettlementID(subjectType string) string {
	switch subjectType {
	case "MICRO":
		return "703"
	case "INDIVIDUAL":
		return "719"
	case "ENTERPRISE":
		return "716"
	default:
		return ""
	}
}

func onboardingIndustryOptions(subjectType string) []onboardingIndustryRequirement {
	settlementID := expectedWechatSettlementID(subjectType)
	if subjectType == "MICRO" {
		result := []onboardingIndustryRequirement{}
		for _, industry := range []string{"餐饮", "零售", "居民生活服务", "休闲娱乐", "交通出行"} {
			result = append(result, onboardingIndustryRequirement{
				SubjectType: "MICRO", SettlementID: settlementID, Industry: industry,
				QualificationMode: "CONDITIONAL", Requirement: "小微进件仅向指定实体行业开放；特殊资质是否提交以服务商平台当前费率结算规则表为准。",
				SourceURL: wechatMicroDocURL,
			})
		}
		return result
	}
	rules := registeredIndustryRules[subjectType]
	result := make([]onboardingIndustryRequirement, len(rules))
	for index, rule := range rules {
		rule.SubjectType = subjectType
		rule.SettlementID = settlementID
		rule.SourceURL = wechatSettlementDocURL
		result[index] = rule
	}
	return result
}

func allOnboardingIndustryOptions() []onboardingIndustryRequirement {
	result := []onboardingIndustryRequirement{}
	for _, subjectType := range []string{"MICRO", "INDIVIDUAL", "ENTERPRISE"} {
		result = append(result, onboardingIndustryOptions(subjectType)...)
	}
	return result
}

func selectedOnboardingIndustryRule(subjectType, settlementID, industry string) (onboardingIndustryRequirement, bool) {
	industry = strings.TrimSpace(industry)
	if subjectType == "MICRO" && strings.TrimSpace(settlementID) == "703" && industry != "" {
		for _, rule := range onboardingIndustryOptions("MICRO") {
			if rule.Industry == industry {
				return rule, true
			}
		}
		return onboardingIndustryRequirement{}, false
	}
	for _, rule := range onboardingIndustryOptions(subjectType) {
		if rule.SettlementID == strings.TrimSpace(settlementID) && rule.Industry == industry {
			return rule, true
		}
	}
	return onboardingIndustryRequirement{}, false
}

func onboardingMediaRequirements(app wechatOnboardingApplication, sensitive wechatOnboardingSensitiveInput) []onboardingMediaRequirement {
	isMicro := app.SubjectType == "MICRO"
	requirements := []onboardingMediaRequirement{}
	if !isMicro {
		requirements = append(requirements, onboardingMediaRequirement{FieldName: "businessLicenseCopy", Label: "营业执照", WechatField: "license_copy", WechatPath: "subject_info.business_license_info.license_copy", Required: true, Requirement: "正面、清晰、四角完整的彩色照片或扫描件。"})
	}
	requirements = append(requirements,
		onboardingMediaRequirement{FieldName: "idCardCopy", Label: "身份证人像面", WechatField: "id_card_copy", WechatPath: "subject_info.identity_info.id_card_info.id_card_copy", Required: true, Requirement: "经营者／法定代表人身份证人像面。"},
		onboardingMediaRequirement{FieldName: "idCardNational", Label: "身份证国徽面", WechatField: "id_card_national", WechatPath: "subject_info.identity_info.id_card_info.id_card_national", Required: true, Requirement: "经营者／法定代表人身份证国徽面。"},
	)
	if isMicro {
		if app.BusinessScene == "MOBILE" {
			requirements = append(requirements,
				onboardingMediaRequirement{FieldName: "storeEntrancePic", Label: "经营现场全景", WechatField: "micro_mobile_pics[0]", WechatPath: "subject_info.micro_biz_info.micro_mobile_info.micro_mobile_pics", Required: true, Requirement: "流动摊位全景或经营服务现场照片。"},
				onboardingMediaRequirement{FieldName: "indoorPic", Label: "商品／服务现场", WechatField: "micro_mobile_pics[1]", WechatPath: "subject_info.micro_biz_info.micro_mobile_info.micro_mobile_pics", Required: true, Requirement: "清晰展示实际售卖商品或服务内容。"},
			)
		} else {
			requirements = append(requirements,
				onboardingMediaRequirement{FieldName: "storeEntrancePic", Label: "门店门头", WechatField: "store_entrance_pic", WechatPath: "subject_info.micro_biz_info.micro_store_info.store_entrance_pic", Required: true, Requirement: "门店招牌、门框完整清晰可辨。"},
				onboardingMediaRequirement{FieldName: "indoorPic", Label: "店内环境", WechatField: "micro_indoor_copy", WechatPath: "subject_info.micro_biz_info.micro_store_info.micro_indoor_copy", Required: true, Requirement: "能够辨识实际经营内容。"},
			)
		}
	} else {
		requirements = append(requirements,
			onboardingMediaRequirement{FieldName: "storeEntrancePic", Label: "门店门头", WechatField: "store_entrance_pic", WechatPath: "business_info.sales_info.biz_store_info.store_entrance_pic", Required: true, Requirement: "门店招牌、门框完整清晰可辨。"},
			onboardingMediaRequirement{FieldName: "indoorPic", Label: "店内环境", WechatField: "indoor_pic", WechatPath: "business_info.sales_info.biz_store_info.indoor_pic", Required: true, Requirement: "能够辨识实际经营内容。"},
			onboardingMediaRequirement{FieldName: "cashierPic", Label: "收银台", WechatField: "indoor_pic", WechatPath: "business_info.sales_info.biz_store_info.indoor_pic", Required: false, Requirement: "餐饮无许可证时，与门头、店内环境照片共同作为替代材料。"},
			onboardingMediaRequirement{FieldName: "miniProgramPic", Label: "小程序经营页面", WechatField: "mini_program_pics", WechatPath: "business_info.sales_info.mini_program_info.mini_program_pics", Required: true, Requirement: "展示实际商品或服务；小程序未上线时应提供设计稿。"},
		)
	}
	rule, known := selectedOnboardingIndustryRule(app.SubjectType, sensitive.SettlementID, sensitive.QualificationType)
	mode := "CONDITIONAL"
	requirement := "仅在所选行业规则要求时提交；无需特殊资质的行业不得传入该字段。"
	required := false
	if known {
		mode, requirement = rule.QualificationMode, rule.Requirement
		required = mode == "REQUIRED"
	}
	requirements = append(requirements, onboardingMediaRequirement{FieldName: "qualificationPic", Label: "行业特殊资质", WechatField: "qualifications", WechatPath: "settlement_info.qualifications", Required: required, Requirement: requirement})
	return requirements
}
