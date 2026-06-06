package utilities

import "regexp"

// idCardPattern matches a valid 18-digit Chinese Resident Identity Card number
// at the format level (area code + DOB + sequence + check digit).
var idCardPattern = regexp.MustCompile(
	`^[1-9]\d{5}(?:18|19|20)\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])\d{3}[\dXx]$`,
)

// dateRE matches YYYY-MM-DD format.
var dateRE = regexp.MustCompile(`^\d{4}-(?:0[1-9]|1[0-2])-(?:0[1-9]|[12]\d|3[01])$`)

// gbWeight factors for the first 17 digits of an 18-digit Chinese ID number.
var gbWeight = [17]int{7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2}

// gbCheckTable maps remainder (sum % 11) to the expected check character.
var gbCheckTable = [11]byte{'1', '0', 'X', '9', '8', '7', '6', '5', '4', '3', '2'}

// regionCodes maps 6-digit administrative division codes to region names.
var regionCodes = map[string]string{
	// North China
	"110000": "北京市",
	"120000": "天津市",
	"130000": "河北省",
	"140000": "山西省",
	"150000": "内蒙古自治区",
	// Northeast
	"210000": "辽宁省",
	"220000": "吉林省",
	"230000": "黑龙江省",
	// East China
	"310000": "上海市",
	"320000": "江苏省",
	"330000": "浙江省",
	"340000": "安徽省",
	"350000": "福建省",
	"360000": "江西省",
	"370000": "山东省",
	// Central China
	"410000": "河南省",
	"420000": "湖北省",
	"430000": "湖南省",
	// South China
	"440000": "广东省",
	"450000": "广西壮族自治区",
	"460000": "海南省",
	// Southwest
	"500000": "重庆市",
	"510000": "四川省",
	"520000": "贵州省",
	"530000": "云南省",
	"540000": "西藏自治区",
	// Northwest
	"610000": "陕西省",
	"620000": "甘肃省",
	"630000": "青海省",
	"640000": "宁夏回族自治区",
	"650000": "新疆维吾尔自治区",
	// Special Administrative Regions
	"810000": "香港特别行政区",
	"820000": "澳门特别行政区",
	// County-level
	"350125": "福建省福州市永泰县",
	"130982": "河北省沧州市任丘市",
	"441721": "广东省阳江市阳西县",
	"210504": "辽宁省本溪市明山区",
	"231201": "黑龙江省绥化市市辖区",
	"362424": "江西省抚州地区南丰县",
	// Jiangxi prefecture-level cities
	"360100": "江西省南昌市",
	"360200": "江西省景德镇市",
	"360300": "江西省萍乡市",
	"360400": "江西省九江市",
	"360500": "江西省新余市",
	"360600": "江西省鹰潭市",
	"360700": "江西省赣州市",
	"360800": "江西省吉安市",
	"360900": "江西省宜春市",
	"361000": "江西省抚州市",
	"361100": "江西省上饶市",
}

// ValidateChineseIDNumber checks whether s matches the 18-digit format regex.
// For full GB11643-1999 checksum validation, use ValidateChineseIDNumberFull.
func ValidateChineseIDNumber(s string) bool {
	return idCardPattern.MatchString(s)
}

// ValidateChineseIDNumberFull performs the complete GB11643-1999 validation on
// an 18-digit Chinese Resident Identity Card number:
//  1. Format check via regex
//  2. Checksum verification using weighted factors
func ValidateChineseIDNumberFull(s string) bool {
	if !idCardPattern.MatchString(s) {
		return false
	}

	upper := []byte(s)
	if upper[17] >= 'a' && upper[17] <= 'z' {
		upper[17] -= 32 // to uppercase
	}
	if upper[17] == 'x' {
		upper[17] = 'X'
	}

	sum := 0
	for i := 0; i < 17; i++ {
		sum += int(upper[i]-'0') * gbWeight[i]
	}

	return gbCheckTable[sum%11] == upper[17]
}

// ValidateDateFormat checks whether s is in YYYY-MM-DD format.
func ValidateDateFormat(s string) bool {
	return dateRE.MatchString(s)
}

// DOBFromIDNumber extracts the date of birth (YYYY-MM-DD) from a valid 18-digit
// Chinese ID number. Returns empty string if the ID is not 18 characters.
func DOBFromIDNumber(id string) string {
	if len(id) != 18 {
		return ""
	}
	return id[6:10] + "-" + id[10:12] + "-" + id[12:14]
}

// ValidateDOBConsistency checks whether dob matches the date-of-birth encoded
// in positions [6..14) of the ID number.
func ValidateDOBConsistency(id, dob string) bool {
	return dob == DOBFromIDNumber(id)
}

// SexFromIDNumber derives sex from a Chinese ID number.
// Returns "男" for odd 17th digit, "女" for even. Returns "" if ID is not 18 chars.
func SexFromIDNumber(id string) string {
	if len(id) != 18 {
		return ""
	}
	if (id[16]-'0')%2 == 1 {
		return "男"
	}
	return "女"
}

// ValidateSexConsistency checks whether sex matches the 17th digit of the ID.
func ValidateSexConsistency(id, sex string) bool {
	return sex == SexFromIDNumber(id)
}

// IDInfo holds parsed fields from a Chinese Resident Identity Card number.
type IDInfo struct {
	Number      string // the full 18-digit ID number
	Region      string // region name (e.g. "福建省福州市永泰县")
	RegionCode  string // 6-digit administrative code
	DateOfBirth string // YYYY-MM-DD
	Sex         string // "男" or "女"
	CheckDigit  string // the check character (0-9 or X)
}

// ParseIDInfo parses an 18-digit Chinese ID number into structured IDInfo.
// Returns nil if the number fails full GB11643-1999 validation.
func ParseIDInfo(id string) *IDInfo {
	if !ValidateChineseIDNumberFull(id) {
		return nil
	}

	info := &IDInfo{
		Number:      id,
		RegionCode:  id[0:6],
		DateOfBirth: DOBFromIDNumber(id),
		Sex:         SexFromIDNumber(id),
		CheckDigit:  string(id[17]),
	}

	if name, ok := regionCodes[info.RegionCode]; ok {
		info.Region = name
	} else {
		// Fall back to provincial-level lookup (first 2 digits + "0000")
		provinceKey := id[0:2] + "0000"
		if name, ok := regionCodes[provinceKey]; ok {
			info.Region = name
		}
	}

	return info
}

// MyKadBirthMonth returns the 3-letter month abbreviation for a two-digit
// month code as encoded in a Malaysian MyKad (e.g. "01" → "JAN").
func MyKadBirthMonth(code string) string {
	switch code {
	case "01":
		return "JAN"
	case "02":
		return "FEB"
	case "03":
		return "MAR"
	case "04":
		return "APR"
	case "05":
		return "MAY"
	case "06":
		return "JUN"
	case "07":
		return "JUL"
	case "08":
		return "AUG"
	case "09":
		return "SEP"
	case "10":
		return "OCT"
	case "11":
		return "NOV"
	case "12":
		return "DEC"
	default:
		return ""
	}
}

// MyKadBirthPlace returns the state or federal territory name for a two-digit
// Malaysian MyKad birth place code (positions 7-8 of the 12-digit MyKad number).
func MyKadBirthPlace(code string) string {
	switch code {
	case "01", "21", "22", "23", "24":
		return "JOHOR"
	case "02", "25", "26", "27":
		return "KEDAH"
	case "03", "28", "29":
		return "KELANTAN"
	case "04", "30":
		return "MALACCA"
	case "05", "31", "59":
		return "NEGERI SEMBILAN"
	case "06", "32", "33":
		return "PAHANG"
	case "07", "34", "35":
		return "PENANG"
	case "08", "36", "37", "38", "39":
		return "PERAK"
	case "09", "40":
		return "PERLIS"
	case "10", "41", "42", "43", "44":
		return "SELANGOR"
	case "11", "45", "46":
		return "TERENGGANU"
	case "12", "47", "48", "49":
		return "SABAH"
	case "13", "50", "51", "52", "53":
		return "SARAWAK"
	case "14", "54", "55", "56", "57":
		return "WILAYAH PERSEKUTUAN KUALA LUMPUR"
	case "15", "58":
		return "WILAYAH PERSEKUTUAN LABUAN"
	case "16":
		return "WILAYAH PERSEKUTUAN PUTRAJAYA"
	default:
		return ""
	}
}
