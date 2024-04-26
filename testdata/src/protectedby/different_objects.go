package protectedby

func differentObjects() {
	p1 := s1{}
	p2 := s1{}

	p1.mu.Lock()
	p2.protectedField1 = 42 // not protected access to shared field protectedField1, use p2.mu.Lock()
}
