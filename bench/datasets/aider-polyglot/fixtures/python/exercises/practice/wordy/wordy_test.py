import unittest

from wordy import answer


class WordyTest(unittest.TestCase):
    def test_just_a_number(self):
        self.assertEqual(answer("What is 5?"), 5)


if __name__ == "__main__":
    unittest.main()
