package main

import (
	"encoding/binary"
	"os"
	"testing"
)

func TestOfficialIconBuildsWindowsResources(t *testing.T) {
	icon, err := os.ReadFile("../../assets/coretuner.ico")
	if err != nil {
		t.Fatal(err)
	}
	images, err := parseICO(icon)
	if err != nil {
		t.Fatal(err)
	}
	if len(images) < 8 {
		t.Fatalf("esperava vários tamanhos de ícone, recebeu %d", len(images))
	}
	resource := buildResourceSection(images, 0x5000)
	if len(resource) < len(icon) {
		t.Fatalf("seção de recursos inesperadamente pequena: %d", len(resource))
	}
	if got := binary.LittleEndian.Uint16(resource[14:16]); got != 2 {
		t.Fatalf("raiz deveria possuir 2 tipos de recurso, recebeu %d", got)
	}
	if got := binary.LittleEndian.Uint32(resource[16:20]); got != resourceTypeIcon {
		t.Fatalf("primeiro recurso deveria ser RT_ICON, recebeu %d", got)
	}
	if got := binary.LittleEndian.Uint32(resource[24:28]); got != resourceTypeGroupIcon {
		t.Fatalf("segundo recurso deveria ser RT_GROUP_ICON, recebeu %d", got)
	}
}
