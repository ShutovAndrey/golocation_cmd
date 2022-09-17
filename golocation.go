package main

import (
	"archive/zip"
	"encoding/csv"
	"fmt"
	"github.com/ShutovAndrey/golocation/logger"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func init() {
	// get .env
	godotenv.Load()
}

type IpAd struct {
	ipNet *net.IPNet
	code  string
}

func (n *IpAd) containsIP(ip net.IP) int8 {
	//customized net.Conteins function

	nn, m := n.ipNet.IP.To4(), n.ipNet.Mask

	if x := ip.To4(); x != nil {
		ip = x
	}

	for i := 0; i < len(ip); i++ {
		if nn[i]&m[i] > ip[i]&m[i] {
			return 1
		}
		if nn[i]&m[i] < ip[i]&m[i] {
			return -1
		}
	}
	return 0
}

func getLocationCodeByIp(ipAdresses *[]IpAd, needleIp net.IP) string {

	arr := *ipAdresses
	start := 0
	end := len(arr) - 1

	//bunary search
	for start <= end {
		mid := (start + end) / 2
		res := arr[mid].containsIP(needleIp)
		switch res {
		case 0:
			return arr[mid].code
		case 1:
			end = mid - 1
		case -1:
			start = mid + 1
		}
	}
	return ""
}

func getFromDB(name string) ([]IpAd, map[string]string) {
	fileNames, err := downloadDB(name)
	if err != nil {
		logger.Error(err)
	} else {
		logger.Info(fmt.Sprintf("database %s successfully downloaded", name))
	}

	ipAdresses, err := readCsvFileIP(fileNames["Blocks-IPv4"])
	if err != nil {
		logger.Error(err)
	}

	var key, value uint8
	if name == "City" {
		key, value = 0, 10
	} else {
		key, value = 0, 5
	}

	locations, err := readCsvFile(fileNames["Locations-en"], key, value)
	if err != nil {
		logger.Error(err)
	}
	return ipAdresses, locations
}

func downloadDB(dbType string) (map[string]string, error) {
	key, ok := os.LookupEnv("MAXMIND_KEY")
	var path, tmpDir string

	if ok {
		uri := fmt.Sprintf(
			"https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-%s-CSV&license_key=%s&suffix=zip",
			dbType, key)

		resp, err := http.Get(uri)
		if err != nil {
			return nil, errors.Wrap(err, "Can't download file")
		}

		if resp.StatusCode != 200 {
			return nil, errors.Errorf("Received non 200 response code")
		}

		defer resp.Body.Close()

		var contentName string
		name, ok := resp.Header["Content-Disposition"]

		if !ok {
			contentName = fmt.Sprintf("GeoLite2-%s-CSV-%s.zip", dbType, time.Now().Format("01022006"))
			logger.Info("No content-desposition header. The default name setted")
		} else {
			contentName = strings.Split(name[0], "filename=")[1]

			if len(contentName) == 0 {
				contentName = fmt.Sprintf("GeoLite2-%s-CSV-%s.zip", dbType, time.Now().Format("01022006"))
				logger.Info("empty contentName. The default name setted")
			}
		}

		tmpDir = os.TempDir()
		path = filepath.Join(tmpDir, contentName)

		out, err := os.Create(path)
		if err != nil {
			return nil, errors.Wrapf(err, "Can't create file %s", path)

		}
		defer out.Close()

		// Change permissions
		err = os.Chmod(path, 0665)
		if err != nil {
			return nil, errors.Wrapf(err, "Can't change permission to file %s", path)
		}

		_, err = io.Copy(out, resp.Body)
		if err != nil {
			return nil, errors.Wrapf(err, "Can't copy to file %s", path)
		}
	} else {
		//if you havent maxmind key
		path = fmt.Sprintf("./assets/GeoLite2-%s-CSV.zip", dbType)
		tmpDir = "./assets"
	}

	files, err := unzip(path, tmpDir, dbType)
	if err != nil {
		return nil, err
	}

	return files, nil
}

func unzip(path, dst, dbType string) (map[string]string, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return nil, errors.Wrap(err, "Can't open archive with files")
	}
	defer archive.Close()

	files := make(map[string]string)

	types := [2]string{"Locations-en", "Blocks-IPv4"}

	for _, f := range archive.File {

		//use only IPv4 ranges and countries'n'cities codes
		if !strings.HasSuffix(f.Name, fmt.Sprintf("GeoLite2-%s-Blocks-IPv4.csv", dbType)) &&
			!strings.HasSuffix(f.Name, fmt.Sprintf("GeoLite2-%s-Locations-en.csv", dbType)) {
			continue
		}

		filePath := filepath.Join(dst, f.Name)

		if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			return nil, errors.Wrapf(err, "Can't create directory %s", filePath)
		}

		dstFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return nil, errors.Wrapf(err, "Can't open file %s", filePath)
		}
		fileInArchive, err := f.Open()
		if err != nil {
			return nil, errors.Wrapf(err, "File is broken %s", f.Name)

		}

		if _, err := io.Copy(dstFile, fileInArchive); err != nil {
			return nil, errors.Wrapf(err, "Can't copy file %s", filePath)
		}

		dstFile.Close()
		fileInArchive.Close()

		for _, t := range types {
			if strings.Contains(f.Name, t) {
				files[t] = filePath
			}
		}
	}
	return files, nil
}

func readCsvFile(filePath string, key, value uint8) (map[string]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ','
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	dict := make(map[string]string)

	for i, record := range records {
		if i == 0 {
			continue
		}

		length := uint8(len(record))
		if key > length || value > length {
			return nil, errors.Errorf("Invalid key-value pair")
		}
		dict[record[key]] = record[value]
	}
	if len(dict) != 0 {
		return dict, nil
	} else {
		return nil, errors.Errorf("Empty map")
	}

}

func readCsvFileIP(filePath string) ([]IpAd, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ','
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var ipAdresses []IpAd

	for i, record := range records {
		if i == 0 {
			continue
		}
		_, ipNet, err := net.ParseCIDR(record[0])
		if err != nil {
			return nil, err
		}

		new := IpAd{ipNet: ipNet, code: record[1]}

		ipAdresses = append(ipAdresses, new)

	}
	if len(ipAdresses) != 0 {
		return ipAdresses, nil
	} else {
		return nil, errors.Errorf("Empty map")
	}

}

func main() {
	logger.CreateLogger()
	defer logger.Close()

	ipCountries, countries := getFromDB("Country")
	ipCities, cities := getFromDB("City")

	fmt.Println("Welcome! Please, type IPv4 addresse to know a location \n 'q' or 'quit' to quit")
	for {
		var expr string

		fmt.Print("insert IP => ")
		_, err := fmt.Scanln(&expr)

		if err != nil {
			fmt.Println("Please, type IPv4 addresse")
			continue
		}

		if expr == "q" || expr == "quit" {
			return
		}

		userIP := net.ParseIP(expr)

		if userIP == nil {
			fmt.Println("Please type valid IPv4 addresse!")
			continue
		}

		country, ok := countries[getLocationCodeByIp(&ipCountries, userIP)]
		if !ok {
			country = "Unknown"
		}

		city, ok := cities[getLocationCodeByIp(&ipCities, userIP)]
		if !ok {
			city = "Unknown"
		}

		fmt.Println(fmt.Sprintf("country : %s \ncity    : %s", country, city))
	}

}
