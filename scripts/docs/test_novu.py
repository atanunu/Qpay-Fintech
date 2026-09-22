"""Regression coverage for notification-service documentation integration."""
import unittest
from pathlib import Path
import check
import governance

class NovuGovernanceTests(unittest.TestCase):
    def test_fifth_service_registered(self):
        self.assertEqual(check.SERVICES['Novu'], 'NOT')
        self.assertIn('Novu', governance.SERVICES)
    def test_esm_requires_service_and_root_docs(self):
        self.assertEqual(len(governance.coupled_changes(['Novu/src/content.mjs'])), 2)
    def test_catalogue_requires_service_and_root_docs(self):
        self.assertEqual(len(governance.coupled_changes(['Novu/catalogue/emails.psv'])), 2)
    def test_coupled_change_is_accepted(self):
        self.assertEqual(governance.coupled_changes(['Novu/src/content.mjs','Novu/README.md','README.md']), [])
    def test_novu_register_has_content_and_open_live_tasks(self):
        registers, errors = check.collect()
        self.assertEqual(errors, [])
        rows = registers['Novu']
        self.assertEqual(len(rows), 27)
        self.assertEqual(sum(row['status']=='Implemented' for row in rows), 6)
        self.assertTrue(any(row['status']=='Partial' for row in rows))
        self.assertTrue(any(row['capability']=='Notification production release acceptance' and row['status']=='Planned' for row in rows))

if __name__ == '__main__':
    unittest.main()
