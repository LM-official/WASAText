package api

// The limits every multipart upload of this package shares:
// setMyPhoto, createGroup and setGroupPhoto all read a body built around a photo,
// so how much of it is held and how much of it is accepted is decided once here and never per handler

// multipartMemory is how much of an upload is kept in memory:
// the rest goes to a temporary file of the system, so a big photo does not become a big allocation
const multipartMemory = 8 << 20 // 8 Megabyte

// multipartOverhead is the room left to the multipart framing (boundaries, part headers) on top of the photo itself,
// so that a photo of exactly MaxPhotoBytes still fits in the body
const multipartOverhead = 1 << 20 // 1 Megabyte
