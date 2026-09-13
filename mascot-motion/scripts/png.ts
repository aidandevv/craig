import { crc32, deflateSync } from "node:zlib";

/** An 8-bit RGBA image, stored row by row, four bytes per pixel. */
export type RgbaImage = {
	readonly width: number;
	readonly height: number;
	readonly data: Uint8Array;
};

const SIGNATURE = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]);

const chunk = (type: string, body: Buffer): Buffer => {
	const length = Buffer.alloc(4);
	length.writeUInt32BE(body.length);
	const typeAndBody = Buffer.concat([Buffer.from(type, "ascii"), body]);
	const crc = Buffer.alloc(4);
	crc.writeUInt32BE(crc32(typeAndBody));
	return Buffer.concat([length, typeAndBody, crc]);
};

/** Encodes an RGBA image as a PNG: 8-bit depth, color type 6, no scanline filtering. */
export const encodePng = ({ data, height, width }: RgbaImage): Buffer => {
	if (data.length !== width * height * 4) {
		throw new Error(`Expected ${width * height * 4} bytes for ${width}x${height}, got ${data.length}.`);
	}
	const header = Buffer.alloc(13);
	header.writeUInt32BE(width, 0);
	header.writeUInt32BE(height, 4);
	header[8] = 8; // bit depth
	header[9] = 6; // color type: RGBA; compression, filter, and interlace stay 0

	const stride = width * 4;
	const scanlines = Buffer.alloc(height * (stride + 1));
	for (let y = 0; y < height; y++) {
		// Each scanline starts with filter type 0 (none), already zeroed by alloc.
		scanlines.set(data.subarray(y * stride, (y + 1) * stride), y * (stride + 1) + 1);
	}

	return Buffer.concat([
		SIGNATURE,
		chunk("IHDR", header),
		chunk("IDAT", deflateSync(scanlines)),
		chunk("IEND", Buffer.alloc(0)),
	]);
};
