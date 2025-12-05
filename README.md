# Pastebinserve

## Useaage

It's simple
2. `curl -X POST -d "PAYLOAD" $URL` - to store, response is $ID
4. `curl -X GET "$URL/$ID"`         - to get back stored things
3. `curl $URL`                      - to get a list of all stored things
5. `curl -X PUT "$URL/$ID"`         - to upsert an existing thing
3. `curl -X DELETE "$URL/$ID"`      - to delete

## Install

There's a makefile.
