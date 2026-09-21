import unittest
from unittest.mock import patch

from scripts import reindex_contracts


def uint256_word(value):
    low = f'{value:064x}'
    return ('0' * (reindex_contracts.ABI_WORD_HEX_LENGTH - len(low))) + low


def dynamic_string(value):
    data = value.encode('utf-8')
    padded_length = (
        (len(data) + reindex_contracts.ABI_WORD_BYTES - 1)
        // reindex_contracts.ABI_WORD_BYTES
        * reindex_contracts.ABI_WORD_BYTES
    )
    padded = data + (b'\x00' * (padded_length - len(data)))
    return '0x' + uint256_word(64) + uint256_word(len(data)) + padded.hex()


class AddressTests(unittest.TestCase):
    LOWER_WALLET_JS_VECTOR = (
        'd5812f6cf4a0f645aa620cd57319a0ed649dd8f5519a9dde7770ae5b0e49e547'
        '985f35eb972a2a07041561aa39c65a3991478f9b1e6749e05277dcf58a9a8b72'
    )
    CHECKSUM_WALLET_JS_VECTOR = (
        'd5812F6Cf4a0f645aa620cd57319a0Ed649dd8f5519A9dde7770ae5b0E49e547'
        '985f35eB972A2a07041561aa39c65A3991478f9B1e6749e05277dcf58A9A8B72'
    )

    def test_normalizes_supported_qip55_spellings(self):
        body = 'a1' * 64
        expected = 'Q' + body
        for address in (
            'Q' + body,
            'q' + body.upper(),
            '0x' + body,
            '0X' + body.upper(),
            bytes.fromhex(body),
            bytearray.fromhex(body),
        ):
            with self.subTest(address_type=type(address).__name__):
                self.assertEqual(reindex_contracts.normalize_address(address), expected)

    def test_accepts_pinned_wallet_js_checksum_vector(self):
        expected_storage = 'Q' + self.LOWER_WALLET_JS_VECTOR
        for address in (
            'Q' + self.LOWER_WALLET_JS_VECTOR,
            'Q' + self.LOWER_WALLET_JS_VECTOR.upper(),
            'Q' + self.CHECKSUM_WALLET_JS_VECTOR,
            'q' + self.CHECKSUM_WALLET_JS_VECTOR,
            '0x' + self.CHECKSUM_WALLET_JS_VECTOR,
            '0X' + self.CHECKSUM_WALLET_JS_VECTOR,
            bytes.fromhex(self.LOWER_WALLET_JS_VECTOR),
        ):
            with self.subTest(prefix=str(address)[:2]):
                self.assertEqual(reindex_contracts.normalize_address(address), expected_storage)

        self.assertEqual(
            reindex_contracts.checksummed_body(self.LOWER_WALLET_JS_VECTOR),
            self.CHECKSUM_WALLET_JS_VECTOR,
        )

    def test_rejects_invalid_mixed_case_checksum(self):
        invalid_checksum = 'D' + self.CHECKSUM_WALLET_JS_VECTOR[1:]
        for address in (
            'Q' + invalid_checksum,
            '0x' + invalid_checksum,
            'Q' + ('Ab' * 64),
        ):
            with self.subTest(address=address[:10]):
                with self.assertRaises(ValueError):
                    reindex_contracts.normalize_address(address)

    def test_rejects_malformed_addresses(self):
        for address in (
            'a' * 128,
            'Q' + ('a' * 40),
            'Q' + ('a' * 127),
            'Q' + ('z' * 128),
            b'\x00' * 63,
            None,
        ):
            with self.subTest(address=address):
                with self.assertRaises(ValueError):
                    reindex_contracts.normalize_address(address)


class AbiScalarTests(unittest.TestCase):
    def test_decodes_uint256_boundaries(self):
        for value in (0, 18, (1 << 256) - 1):
            with self.subTest(value=value):
                self.assertEqual(
                    reindex_contracts.decode_uint256_word('0x' + uint256_word(value)),
                    value,
                )

    def test_rejects_noncanonical_scalar_words(self):
        malformed = (
            '0x' + ('0' * 64),
            '0x' + uint256_word(1)[:-1],
            '0x' + uint256_word(1) + uint256_word(2),
            '0x1' + ('0' * 127),
            '0x' + ('0' * 127) + 'z',
            uint256_word(1),
        )
        for value in malformed:
            with self.subTest(length=len(value)):
                with self.assertRaises(ValueError):
                    reindex_contracts.decode_uint256_word(value)


class AbiStringTests(unittest.TestCase):
    def test_decodes_dynamic_strings_across_word_boundary(self):
        for value in ('QTK', 'x' * 64, 'x' * 65):
            with self.subTest(length=len(value)):
                self.assertEqual(reindex_contracts.decode_token_string(dynamic_string(value)), value)

    def test_decodes_fixed_word_string(self):
        encoded = b'Legacy' + (b'\x00' * (reindex_contracts.ABI_WORD_BYTES - 6))
        self.assertEqual(reindex_contracts.decode_token_string('0x' + encoded.hex()), 'Legacy')

    def test_rejects_legacy_and_malformed_dynamic_layouts(self):
        valid = dynamic_string('QTK')
        malformed = (
            '0x' + uint256_word(32) + valid[2 + reindex_contracts.ABI_WORD_HEX_LENGTH:],
            valid[:-2],
            valid + uint256_word(0),
            valid[:-1] + '1',
        )
        for value in malformed:
            with self.subTest(length=len(value)):
                with self.assertRaises(ValueError):
                    reindex_contracts.decode_token_string(value)


class TokenInfoTests(unittest.TestCase):
    def test_uses_qip55_words_and_canonical_contract_address(self):
        body = 'ab' * 64
        responses = {
            '0x06fdde03': dynamic_string('Quantum Token'),
            '0x95d89b41': dynamic_string('QTK'),
            '0x313ce567': '0x' + uint256_word(18),
        }

        def call(contract_address, method_signature):
            self.assertEqual(contract_address, 'Q' + body)
            return responses[method_signature]

        with patch.object(reindex_contracts, 'call_contract_method', side_effect=call):
            self.assertEqual(
                reindex_contracts.get_token_info('0X' + body.upper()),
                ('Quantum Token', 'QTK', 18, True),
            )

    def test_rejects_decimals_above_uint8(self):
        body = 'ab' * 64
        responses = {
            '0x06fdde03': None,
            '0x95d89b41': None,
            '0x313ce567': '0x' + uint256_word(256),
        }
        with patch.object(
            reindex_contracts,
            'call_contract_method',
            side_effect=lambda _, method_signature: responses[method_signature],
        ):
            self.assertEqual(
                reindex_contracts.get_token_info('Q' + body),
                ('', '', 0, False),
            )


if __name__ == '__main__':
    unittest.main()
