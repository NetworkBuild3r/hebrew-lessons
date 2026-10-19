// Package tanakh loads the public-domain Tanakh into tanakh_verses.
//
// Hebrew is Tanach with Nikkud. English is JPS 1917. Both are Public
// Domain. A file whose license is anything else is refused. The shipped
// corpus is 23,206 verses in 39 books. Any other total is refused unless
// the caller passes Expect for a test fixture.
//
// he is stored pointed, in NFC. he_plain is units.Plain of that Hebrew:
// the same consonant skeleton the units validator uses.
package tanakh
