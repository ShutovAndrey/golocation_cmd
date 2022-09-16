package main

import (
	"github.com/stretchr/testify/require"
	"net"
	"os"
	"testing"
)

func TestContainsIP(t *testing.T) {

	_, ipNet, _ := net.ParseCIDR("167.114.238.0/24")

	ipAd := IpAd{ipNet, "3017382"}

	ip := net.ParseIP("167.114.238.22")
	ipFalse := net.ParseIP("167.114.239.22")

	t.Run("GetContainsIp", func(t *testing.T) {
		value := ipAd.containsIP(ip)
		require.Equal(t, int8(0), value)
	})

	t.Run("GetNotContainsIp", func(t *testing.T) {
		value := ipAd.containsIP(ipFalse)
		require.Equal(t, int8(-1), value)
	})
}

func TestGetLocationCodeByIp(t *testing.T) {

	ipNetStrings := [3]string{
		"167.114.238.0/24",
		"195.238.78.0/23",
		"85.235.192.0/19"}

	var ipNets [3]*net.IPNet

	for i, ipn := range ipNetStrings {
		_, ipNet, _ := net.ParseCIDR(ipn)
		ipNets[i] = ipNet
	}

	adresses := []IpAd{
		{ipNets[0], "3017382"},
		{ipNets[1], "2750405"},
		{ipNets[2], "2017370"},
	}

	ip := net.ParseIP("167.114.238.22")
	ipFalse := net.ParseIP("167.114.239.22")

	t.Run("GetExistedLocation", func(t *testing.T) {
		value := getLocationCodeByIp(&adresses, ip)
		require.Equal(t, "3017382", value)
	})

	t.Run("GetNotExistedLocation", func(t *testing.T) {
		value := getLocationCodeByIp(&adresses, ipFalse)
		require.Equal(t, "", value)
	})
}

func TestUnzip(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		files, err := unzip("./assets/GeoLite2-Country-CSV.zip", "./assets", "Country")
		require.NoError(t, err)
		require.Greater(t, len(files), 0)
	})

	t.Run("Error", func(t *testing.T) {
		_, err := unzip("./assets/GeoLite2-Country777-CSV.zip", "./assets", "Country")
		require.ErrorContains(t, err, "Can't open archive with files: open ./assets/GeoLite2-Country777-CSV.zip: no such file or directory", "")
	})
}

func TestReadCsvFile(t *testing.T) {
	//files at ./assets is static so we know their names
	if _, err := os.Stat("GeoLite2-Country-CSV_20220913/GeoLite2-Country-Locations-en.csv"); err != nil {
		unzip("./assets/GeoLite2-Country-CSV.zip", "./assets", "Country")
	}
	t.Run("Success", func(t *testing.T) {
		ipMap, err := readCsvFile("./assets/GeoLite2-Country-CSV_20220913/GeoLite2-Country-Locations-en.csv", 0, 5)
		require.NoError(t, err)
		require.Greater(t, len(ipMap), 0)
	})

	t.Run("Error", func(t *testing.T) {
		_, err := readCsvFile("./assets/GeoLite2-Country-CSV_20220913/GeoLite2-Country-Locations-en.csv", 0, 18)
		require.ErrorContains(t, err, "Invalid key-value pair", "")
	})
}
