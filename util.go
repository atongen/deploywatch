package main

func StringSliceToPointers(strs []string) []*string {
	var ptrs []*string = make([]*string, len(strs))

	for i := 0; i < len(strs); i++ {
		ptrs[i] = &strs[i]
	}

	return ptrs
}

func PointerSliceToStrings(ptrs []*string) []string {
	var strs []string = make([]string, len(ptrs))

	for i := 0; i < len(ptrs); i++ {
		strs[i] = *ptrs[i]
	}

	return strs
}

func PadRight(str, pad string, length int) string {
	for {
		str += pad
		if len(str) > length {
			return str[0:length]
		}
	}
}

func PadLeft(str, pad string, length int) string {
	for {
		str = pad + str
		if len(str) > length {
			return str[0:length]
		}
	}
}
