package main

import (
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

const (
	pe32PlusMagic                 = 0x20b
	resourceDirectoryIndex        = 2
	resourceTypeIcon       uint32 = 3
	resourceTypeGroupIcon  uint32 = 14
	resourceLanguage       uint32 = 0x0409
	resourceSectionFlags   uint32 = 0x40000040 // initialized data | read
)

type iconImage struct {
	width      byte
	height     byte
	colorCount byte
	planes     uint16
	bitCount   uint16
	data       []byte
}

type peLayout struct {
	peOffset          int
	coffOffset        int
	optionalOffset    int
	sectionTable      int
	numberOfSections  uint16
	optionalSize      uint16
	sectionAlignment  uint32
	fileAlignment     uint32
	firstRawOffset    uint32
	maxVirtualEnd     uint32
	maxRawEnd         uint32
	resourceDirectory int
}

func main() {
	exePath := flag.String("exe", "", "caminho do executável Windows que receberá o ícone")
	icoPath := flag.String("ico", "", "caminho do arquivo .ico")
	flag.Parse()
	if *exePath == "" || *icoPath == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := patchExecutable(*exePath, *icoPath); err != nil {
		fmt.Fprintln(os.Stderr, "iconpatch:", err)
		os.Exit(1)
	}
	fmt.Printf("Ícone incorporado em %s\n", filepath.Base(*exePath))
}

func patchExecutable(exePath, icoPath string) error {
	executable, err := os.ReadFile(exePath)
	if err != nil {
		return fmt.Errorf("não foi possível ler %s: %w", exePath, err)
	}
	iconFile, err := os.ReadFile(icoPath)
	if err != nil {
		return fmt.Errorf("não foi possível ler %s: %w", icoPath, err)
	}
	images, err := parseICO(iconFile)
	if err != nil {
		return fmt.Errorf("ícone inválido: %w", err)
	}
	patched, err := addResourceSection(executable, images)
	if err != nil {
		return err
	}
	info, err := os.Stat(exePath)
	if err != nil {
		return err
	}
	temporary := exePath + ".iconpatch.tmp"
	if err := os.WriteFile(temporary, patched, info.Mode()); err != nil {
		return fmt.Errorf("não foi possível gravar arquivo temporário: %w", err)
	}
	if err := os.Remove(exePath); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("não foi possível substituir o executável: %w", err)
	}
	if err := os.Rename(temporary, exePath); err != nil {
		return fmt.Errorf("não foi possível concluir a substituição: %w", err)
	}
	return nil
}

func parseICO(data []byte) ([]iconImage, error) {
	if len(data) < 6 || binary.LittleEndian.Uint16(data[0:2]) != 0 || binary.LittleEndian.Uint16(data[2:4]) != 1 {
		return nil, errors.New("cabeçalho ICO ausente")
	}
	count := int(binary.LittleEndian.Uint16(data[4:6]))
	if count == 0 || count > 256 || len(data) < 6+count*16 {
		return nil, errors.New("quantidade de imagens inválida")
	}
	images := make([]iconImage, 0, count)
	for index := 0; index < count; index++ {
		offset := 6 + index*16
		size := binary.LittleEndian.Uint32(data[offset+8 : offset+12])
		position := binary.LittleEndian.Uint32(data[offset+12 : offset+16])
		end := uint64(position) + uint64(size)
		if size == 0 || end > uint64(len(data)) {
			return nil, fmt.Errorf("imagem %d ultrapassa o arquivo", index+1)
		}
		planes := binary.LittleEndian.Uint16(data[offset+4 : offset+6])
		bitCount := binary.LittleEndian.Uint16(data[offset+6 : offset+8])
		if planes == 0 {
			planes = 1
		}
		if bitCount == 0 {
			bitCount = 32
		}
		payload := make([]byte, size)
		copy(payload, data[position:uint32(end)])
		images = append(images, iconImage{
			width:      data[offset],
			height:     data[offset+1],
			colorCount: data[offset+2],
			planes:     planes,
			bitCount:   bitCount,
			data:       payload,
		})
	}
	return images, nil
}

func addResourceSection(executable []byte, images []iconImage) ([]byte, error) {
	layout, err := inspectPE(executable)
	if err != nil {
		return nil, err
	}
	if readUint32(executable, layout.resourceDirectory) != 0 || readUint32(executable, layout.resourceDirectory+4) != 0 {
		return nil, errors.New("o executável já possui recursos; compile-o novamente antes de aplicar o ícone")
	}
	newHeaderEnd := layout.sectionTable + int(layout.numberOfSections+1)*40
	if uint32(newHeaderEnd) > layout.firstRawOffset {
		return nil, errors.New("não há espaço no cabeçalho PE para a seção de recursos")
	}
	sectionRVA := align(layout.maxVirtualEnd, layout.sectionAlignment)
	resource := buildResourceSection(images, sectionRVA)
	rawSize := align(uint32(len(resource)), layout.fileAlignment)
	rawOffset := align(maxUint32(uint32(len(executable)), layout.maxRawEnd), layout.fileAlignment)

	result := make([]byte, rawOffset+rawSize)
	copy(result, executable)
	copy(result[rawOffset:], resource)

	sectionHeader := layout.sectionTable + int(layout.numberOfSections)*40
	copy(result[sectionHeader:sectionHeader+8], []byte{'.', 'r', 's', 'r', 'c', 0, 0, 0})
	writeUint32(result, sectionHeader+8, uint32(len(resource)))
	writeUint32(result, sectionHeader+12, sectionRVA)
	writeUint32(result, sectionHeader+16, rawSize)
	writeUint32(result, sectionHeader+20, rawOffset)
	writeUint32(result, sectionHeader+36, resourceSectionFlags)

	binary.LittleEndian.PutUint16(result[layout.coffOffset+2:layout.coffOffset+4], layout.numberOfSections+1)
	initializedData := readUint32(result, layout.optionalOffset+8)
	writeUint32(result, layout.optionalOffset+8, initializedData+rawSize)
	writeUint32(result, layout.optionalOffset+56, align(sectionRVA+uint32(len(resource)), layout.sectionAlignment))
	writeUint32(result, layout.resourceDirectory, sectionRVA)
	writeUint32(result, layout.resourceDirectory+4, uint32(len(resource)))
	writeUint32(result, layout.optionalOffset+64, 0) // checksum opcional; zero é aceito para executáveis não assinados
	return result, nil
}

func inspectPE(data []byte) (peLayout, error) {
	if len(data) < 0x100 || string(data[0:2]) != "MZ" {
		return peLayout{}, errors.New("arquivo não é um executável PE")
	}
	peOffset := int(readUint32(data, 0x3c))
	if peOffset < 0x40 || peOffset+24 > len(data) || string(data[peOffset:peOffset+4]) != "PE\x00\x00" {
		return peLayout{}, errors.New("assinatura PE inválida")
	}
	coffOffset := peOffset + 4
	numberOfSections := binary.LittleEndian.Uint16(data[coffOffset+2 : coffOffset+4])
	optionalSize := binary.LittleEndian.Uint16(data[coffOffset+16 : coffOffset+18])
	optionalOffset := coffOffset + 20
	sectionTable := optionalOffset + int(optionalSize)
	if numberOfSections == 0 || sectionTable+int(numberOfSections)*40 > len(data) {
		return peLayout{}, errors.New("tabela de seções inválida")
	}
	if binary.LittleEndian.Uint16(data[optionalOffset:optionalOffset+2]) != pe32PlusMagic {
		return peLayout{}, errors.New("apenas executáveis Windows PE32+ de 64 bits são aceitos")
	}
	sectionAlignment := readUint32(data, optionalOffset+32)
	fileAlignment := readUint32(data, optionalOffset+36)
	if sectionAlignment == 0 || fileAlignment == 0 {
		return peLayout{}, errors.New("alinhamento PE inválido")
	}
	firstRaw := uint32(^uint32(0))
	var maxVirtualEnd uint32
	var maxRawEnd uint32
	for index := 0; index < int(numberOfSections); index++ {
		header := sectionTable + index*40
		virtualSize := readUint32(data, header+8)
		virtualAddress := readUint32(data, header+12)
		rawSize := readUint32(data, header+16)
		rawOffset := readUint32(data, header+20)
		if rawOffset != 0 && rawOffset < firstRaw {
			firstRaw = rawOffset
		}
		virtualEnd := virtualAddress + align(maxUint32(virtualSize, rawSize), sectionAlignment)
		if virtualEnd > maxVirtualEnd {
			maxVirtualEnd = virtualEnd
		}
		if rawOffset+rawSize > maxRawEnd {
			maxRawEnd = rawOffset + rawSize
		}
	}
	if firstRaw == ^uint32(0) {
		return peLayout{}, errors.New("nenhuma seção física encontrada")
	}
	return peLayout{
		peOffset:          peOffset,
		coffOffset:        coffOffset,
		optionalOffset:    optionalOffset,
		sectionTable:      sectionTable,
		numberOfSections:  numberOfSections,
		optionalSize:      optionalSize,
		sectionAlignment:  sectionAlignment,
		fileAlignment:     fileAlignment,
		firstRawOffset:    firstRaw,
		maxVirtualEnd:     maxVirtualEnd,
		maxRawEnd:         maxRawEnd,
		resourceDirectory: optionalOffset + 112 + resourceDirectoryIndex*8,
	}, nil
}

func buildResourceSection(images []iconImage, sectionRVA uint32) []byte {
	count := len(images)
	rootOffset := uint32(0)
	rootSize := uint32(16 + 2*8)
	iconTypeOffset := rootOffset + rootSize
	iconTypeSize := uint32(16 + count*8)
	iconLanguagesOffset := iconTypeOffset + iconTypeSize
	languageDirectorySize := uint32(24)
	groupTypeOffset := iconLanguagesOffset + uint32(count)*languageDirectorySize
	groupLanguageOffset := groupTypeOffset + 24
	dataEntriesOffset := groupLanguageOffset + 24
	iconDataEntriesOffset := dataEntriesOffset
	groupDataEntryOffset := iconDataEntriesOffset + uint32(count)*16
	blobsOffset := align(groupDataEntryOffset+16, 4)

	iconBlobOffsets := make([]uint32, count)
	position := blobsOffset
	for index, image := range images {
		position = align(position, 4)
		iconBlobOffsets[index] = position
		position += uint32(len(image.data))
	}
	position = align(position, 4)
	groupBlobOffset := position
	groupData := buildGroupIcon(images)
	position += uint32(len(groupData))

	result := make([]byte, position)
	writeDirectory := func(offset uint32, idEntries uint16) {
		binary.LittleEndian.PutUint16(result[offset+14:offset+16], idEntries)
	}
	writeEntry := func(offset, id, target uint32, directory bool) {
		writeUint32(result, int(offset), id)
		if directory {
			target |= 0x80000000
		}
		writeUint32(result, int(offset+4), target)
	}

	writeDirectory(rootOffset, 2)
	writeEntry(16, resourceTypeIcon, iconTypeOffset, true)
	writeEntry(24, resourceTypeGroupIcon, groupTypeOffset, true)

	writeDirectory(iconTypeOffset, uint16(count))
	for index := range images {
		languageOffset := iconLanguagesOffset + uint32(index)*languageDirectorySize
		writeEntry(iconTypeOffset+16+uint32(index)*8, uint32(index+1), languageOffset, true)
		writeDirectory(languageOffset, 1)
		writeEntry(languageOffset+16, resourceLanguage, iconDataEntriesOffset+uint32(index)*16, false)
	}

	writeDirectory(groupTypeOffset, 1)
	writeEntry(groupTypeOffset+16, 1, groupLanguageOffset, true)
	writeDirectory(groupLanguageOffset, 1)
	writeEntry(groupLanguageOffset+16, resourceLanguage, groupDataEntryOffset, false)

	for index, image := range images {
		blobOffset := iconBlobOffsets[index]
		dataEntry := iconDataEntriesOffset + uint32(index)*16
		writeUint32(result, int(dataEntry), sectionRVA+blobOffset)
		writeUint32(result, int(dataEntry+4), uint32(len(image.data)))
		copy(result[blobOffset:blobOffset+uint32(len(image.data))], image.data)
	}
	writeUint32(result, int(groupDataEntryOffset), sectionRVA+groupBlobOffset)
	writeUint32(result, int(groupDataEntryOffset+4), uint32(len(groupData)))
	copy(result[groupBlobOffset:groupBlobOffset+uint32(len(groupData))], groupData)
	return result
}

func buildGroupIcon(images []iconImage) []byte {
	result := make([]byte, 6+len(images)*14)
	binary.LittleEndian.PutUint16(result[2:4], 1)
	binary.LittleEndian.PutUint16(result[4:6], uint16(len(images)))
	for index, image := range images {
		offset := 6 + index*14
		result[offset] = image.width
		result[offset+1] = image.height
		result[offset+2] = image.colorCount
		binary.LittleEndian.PutUint16(result[offset+4:offset+6], image.planes)
		binary.LittleEndian.PutUint16(result[offset+6:offset+8], image.bitCount)
		binary.LittleEndian.PutUint32(result[offset+8:offset+12], uint32(len(image.data)))
		binary.LittleEndian.PutUint16(result[offset+12:offset+14], uint16(index+1))
	}
	return result
}

func align(value, alignment uint32) uint32 {
	return (value + alignment - 1) / alignment * alignment
}

func maxUint32(first, second uint32) uint32 {
	if first > second {
		return first
	}
	return second
}

func readUint32(data []byte, offset int) uint32 {
	return binary.LittleEndian.Uint32(data[offset : offset+4])
}

func writeUint32(data []byte, offset int, value uint32) {
	binary.LittleEndian.PutUint32(data[offset:offset+4], value)
}
