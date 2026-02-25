all: clear rb2300-docs

clear:
	rm -rf ./_compiled/*

rb2300-docs:
	wget https://www.railbox.pl/_files/ugd/6c739b_223b9a94c9d94cd18887f7087ab4c124.pdf -O /tmp/rb2300-docs.pdf
	pdftotext -layout /tmp/rb2300-docs.pdf ./_compiled/rb2300-docs.txt
