package netease

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

// --- 常量定义 ---

// Linux API Key (Hex)
const linuxApiKeyHex = "7246674226682325323F5E6544673A51"

// WeApi Constants
const (
	weApiNonce      = "0CoJUm6Qyw8W8jud"
	weApiIv         = "0102030405060708"
	weApiPubModulus = "00e0b509f6259df8642dbc35662901477df22677ec152b5ff68ace615bb7b725152b3ab17a876aea8a5aa76d2e417629ec4ee341f56135fccf695280104e0312ecbda92557c93870114af6c9d05c4f7f0c3685b7a46bee255932575cce10b424d813cfe4875d3e82047b97ddef52741d546b8e289dc6935b3ece0462db0a22b8e7"
	weApiPubKey     = "010001"
)

// --- 辅助函数 ---

// pkcs7Padding 填充
func pkcs7Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

// randomString 生成指定长度随机字符串
func randomString(size int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var result string
	b := make([]byte, size)
	rand.Read(b)
	for _, v := range b {
		result += string(letters[int(v)%len(letters)])
	}
	return result
}

// reverseString 反转字符串 (RSA加密需要)
func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// --- 算法实现 ---

// aesEncryptECB 实现 AES-128-ECB 加密 (Go 标准库没有直接提供 ECB，需手动实现)
func aesEncryptECB(origData []byte, key []byte) []byte {
	block, _ := aes.NewCipher(key)
	// 补码
	origData = pkcs7Padding(origData, block.BlockSize())
	
	crypted := make([]byte, len(origData))
	// 手动循环加密每个块
	for bs, be := 0, block.BlockSize(); bs < len(origData); bs, be = bs+block.BlockSize(), be+block.BlockSize() {
		block.Encrypt(crypted[bs:be], origData[bs:be])
	}
	return crypted
}

// aesEncryptCBC 实现 AES-128-CBC 加密
func aesEncryptCBC(text string, key string, iv string) string {
	keyBytes := []byte(key)
	ivBytes := []byte(iv)
	srcBytes := []byte(text)

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return ""
	}

	srcBytes = pkcs7Padding(srcBytes, block.BlockSize())
	blockMode := cipher.NewCBCEncrypter(block, ivBytes)
	crypted := make([]byte, len(srcBytes))
	blockMode.CryptBlocks(crypted, srcBytes)

	return base64.StdEncoding.EncodeToString(crypted)
}

// rsaEncrypt 实现 RSA 加密 (NoPadding)
// Python: pow(int(hex(text)), int(pub), int(mod))
func rsaEncrypt(text, pubKey, modulus string) string {
	// 1. 反转字符串
	text = reverseString(text)
	// 2. 转为 hex
	hexText := hex.EncodeToString([]byte(text))
	
	// 3. 大数运算
	biText := new(big.Int)
	biText.SetString(hexText, 16)

	biPub := new(big.Int)
	biPub.SetString(pubKey, 16)

	biMod := new(big.Int)
	biMod.SetString(modulus, 16)

	// exp = text^pub % mod
	biRet := new(big.Int).Exp(biText, biPub, biMod)
	
	// 4. 补齐 256 位 hex
	return fmt.Sprintf("%0256x", biRet)
}

// --- 对外暴露的加密方法 ---

// EncryptLinux 对应 Python: encode_netease_data
// 用于搜索接口
func EncryptLinux(data string) string {
	key, _ := hex.DecodeString(linuxApiKeyHex)
	encrypted := aesEncryptECB([]byte(data), key)
	return strings.ToUpper(hex.EncodeToString(encrypted))
}

// EncryptWeApi 对应 Python: encrypted_request
// 用于下载接口
func EncryptWeApi(text string) (string, string) {
	// 1. 生成随机 16 位 secKey
	secKey := randomString(16)

	// 2. 第一次 AES 加密 (Text + Nonce)
	encText := aesEncryptCBC(text, weApiNonce, weApiIv)

	// 3. 第二次 AES 加密 (第一次结果 + secKey)
	params := aesEncryptCBC(encText, secKey, weApiIv)

	// 4. RSA 加密 secKey
	encSecKey := rsaEncrypt(secKey, weApiPubKey, weApiPubModulus)

	return params, encSecKey
}
