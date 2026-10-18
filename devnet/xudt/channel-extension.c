/* Test-only xUDT extension: transfers require both a witness proof and metadata
 * on every output. Owner-mode minting seeds the fixture without either. */
#include <stddef.h>
#include <stdint.h>

typedef struct { const uint8_t *p; size_t n; } Slice;
static uint32_t le32(const uint8_t *p) {
  return (uint32_t)p[0] | ((uint32_t)p[1]<<8) | ((uint32_t)p[2]<<16) | ((uint32_t)p[3]<<24);
}
static Slice field(Slice s, size_t index) {
  Slice empty = {0,0};
  if (s.n < 8 || le32(s.p) != s.n) return empty;
  uint32_t first=le32(s.p+4);
  if (first<8 || first%4 || first>s.n || index>=first/4-1) return empty;
  uint32_t start=le32(s.p+4*(index+1));
  uint32_t end=index+1<first/4-1 ? le32(s.p+4*(index+2)) : s.n;
  if (start<first || end<start || end>s.n) return empty;
  return (Slice){s.p+start,end-start};
}
static Slice unpack(Slice s) {
  if (s.n<4 || le32(s.p)!=s.n-4) return (Slice){0,0};
  return (Slice){s.p+4,s.n-4};
}
static int equals(Slice s, const char *text, size_t size) {
  if (s.n!=size) return 0;
  for(size_t i=0;i<size;i++) if(s.p[i]!=(uint8_t)text[i]) return 0;
  return 1;
}
static long load(long code, void *buf, uint64_t *len, size_t index, uint64_t source) {
  register long a0 __asm__("a0")=(long)buf;
  register long a1 __asm__("a1")=(long)len;
  register long a2 __asm__("a2")=0;
  register long a3 __asm__("a3")=index;
  register long a4 __asm__("a4")=source;
  register long a5 __asm__("a5")=0;
  register long a7 __asm__("a7")=code;
  __asm__ volatile("ecall" : "+r"(a0) : "r"(a1),"r"(a2),"r"(a3),"r"(a4),"r"(a5),"r"(a7) : "memory");
  return a0;
}
__attribute__((visibility("default")))
int validate(int owner, size_t index, const uint8_t *args, size_t length) {
  if(index!=0 || length!=1 || args[0]!=42) return 99;
  if(owner) return 0;
  uint8_t buf[2048]; uint64_t len=sizeof(buf);
  if(load(2074,buf,&len,0,0x0100000000000001ULL)!=0 || len>sizeof(buf)) return 100;
  Slice witness=unpack(field((Slice){buf,len},1));
  Slice proof=unpack(field(field(witness,3),0));
  if(!equals(proof,"channel-proof",13)) return 101;
  for(size_t i=0;;i++) {
    len=sizeof(buf);
    long ret=load(2092,buf,&len,i,0x0100000000000002ULL);
    if(ret==1) break;
    if(ret!=0 || len>sizeof(buf) || len<16) return 102;
    Slice data=unpack(field(field((Slice){buf+16,len-16},1),0));
    if(!equals(data,"channel-data",12)) return 103;
  }
  return 0;
}
