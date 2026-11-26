package consts

var DATA_DIR string

func SetDataDir(dir string) {
	DATA_DIR = dir
}

func GetDataDir() string {
	return DATA_DIR
}
