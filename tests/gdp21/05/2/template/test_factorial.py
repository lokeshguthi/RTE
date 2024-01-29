import unittest
from fakultaet import *

class TestFakultaet(unittest.TestCase):

    def test_iter0(self):
        self.assertEqual(fak(0), 1)
    
    def test_iter1(self):
        self.assertEqual(fak(1), 1)

    def test_iter2(self):
        self.assertEqual(fak(2), 2)

    def test_iter10(self):
        self.assertEqual(fak(10), 3628800)

    def test_rek0(self):
        self.assertEqual(fak_rek(0), 1)
    
    def test_rek1(self):
        self.assertEqual(fak_rek(1), 1)

    def test_rek2(self):
        self.assertEqual(fak_rek(2), 2)

    def test_rek10(self):
        self.assertEqual(fak_rek(10), 3628800)