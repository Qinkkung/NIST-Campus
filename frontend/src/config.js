export const NIST_ADDRESS = "0xEC08895F6C21f17b9b4D32b5bE3CcAA79b0E58ED";

// ใช้ ABI แบบย่อ (Human-Readable ABI) ของ ethers.js
export const NIST_ABI = [
  "function balanceOf(address owner) view returns (uint256)",
  "function transfer(address to, uint amount) returns (bool)",
  "function decimals() view returns (uint8)",
  "function symbol() view returns (string)"
];