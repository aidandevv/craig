import assert from "node:assert/strict";
import { test } from "node:test";
import { crc32, inflateSync } from "node:zlib";
import { encodePng } from "./png.ts";

const readChunks = (png: Buffer): { type: string; body: Buffer }[] => {
	const chunks: { type: string; body: Buffer }[] = [];
	let offset = 8;
	while (offset < png.length) {
		const length = png.readUInt32BE(offset);
		const type = png.toString("ascii", offset + 4, offset + 8);
		const body = png.subarray(offset + 8, offset + 8 + length);
		assert.equal(png.readUInt32BE(offset + 8 + length), crc32(png.subarray(offset + 4, offset + 8 + length)), `${type} CRC`);
		chunks.push({ body, type });
		offset += 12 + length;
	}
	return chunks;
};

test("encodes a 2x1 RGBA image as a valid PNG", () => {
	const png = encodePng({ data: Uint8Array.from([255, 0, 0, 255, 0, 0, 255, 128]), height: 1, width: 2 });

	assert.deepEqual([...png.subarray(0, 8)], [0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);
	const chunks = readChunks(png);
	assert.deepEqual(
		chunks.map((chunk) => chunk.type),
		["IHDR", "IDAT", "IEND"],
	);

	const header = chunks[0].body;
	assert.equal(header.readUInt32BE(0), 2);
	assert.equal(header.readUInt32BE(4), 1);
	assert.equal(header[8], 8);
	assert.equal(header[9], 6);

	assert.deepEqual([...inflateSync(chunks[1].body)], [0, 255, 0, 0, 255, 0, 0, 255, 128]);
});

test("rejects data that doesn't match the dimensions", () => {
	assert.throws(() => encodePng({ data: new Uint8Array(4), height: 2, width: 2 }), /Expected 16 bytes/);
});
