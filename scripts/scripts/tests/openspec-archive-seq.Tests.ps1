$scriptUnderTest = Join-Path $PSScriptRoot "..\openspec-archive-seq.ps1"
. $scriptUnderTest

Describe "openspec-archive-seq helpers" {
    It "extracts slug names from sequence and date archive directories" {
        Get-SlugFromArchiveDirName -Name "001-sample-change" | Should Be "sample-change"
        Get-SlugFromArchiveDirName -Name "2026-05-29-sample-change" | Should Be "sample-change"
        Get-SlugFromArchiveDirName -Name "sample-change" | Should Be "sample-change"
    }

    It "extracts numeric sequence prefixes when present" {
        Get-SeqFromArchiveDirName -Name "001-sample-change" | Should Be 1
        Get-SeqFromArchiveDirName -Name "123-sample-change" | Should Be 123
        Get-SeqFromArchiveDirName -Name "2026-05-29-sample-change" | Should Be $null
    }

    It "calculates the next archive sequence from existing numeric entries" {
        $archiveRoot = Join-Path $TestDrive "archive-next-seq"
        New-Item -ItemType Directory -Path $archiveRoot | Out-Null
        New-Item -ItemType Directory -Path (Join-Path $archiveRoot "001-alpha") | Out-Null
        New-Item -ItemType Directory -Path (Join-Path $archiveRoot "004-delta") | Out-Null
        New-Item -ItemType Directory -Path (Join-Path $archiveRoot "2026-05-29-legacy") | Out-Null

        Get-NextSequence -ArchiveRoot $archiveRoot | Should Be 5
    }

    It "migrates existing archive directories to incrementing numeric prefixes" {
        $archiveRoot = Join-Path $TestDrive "archive-migrate"
        New-Item -ItemType Directory -Path $archiveRoot | Out-Null

        $first = New-Item -ItemType Directory -Path (Join-Path $archiveRoot "2026-05-29-first-change")
        $second = New-Item -ItemType Directory -Path (Join-Path $archiveRoot "2026-05-30-second-change")

        $first.LastWriteTime = Get-Date "2026-05-29T10:00:00"
        $second.LastWriteTime = Get-Date "2026-05-29T11:00:00"

        Migrate-ExistingArchive -ArchiveRoot $archiveRoot

        Test-Path (Join-Path $archiveRoot "001-first-change") | Should Be $true
        Test-Path (Join-Path $archiveRoot "002-second-change") | Should Be $true
    }

    It "renames a freshly archived date-based directory to the next sequence and removes the active change directory" {
        $repoRoot = Join-Path $TestDrive "repo"
        $changesRoot = Join-Path $repoRoot "openspec\changes"
        $archiveRoot = Join-Path $changesRoot "archive"
        $activeDir = Join-Path $changesRoot "sample-change"
        $dateArchiveDir = Join-Path $archiveRoot "2026-05-29-sample-change"

        New-Item -ItemType Directory -Path $archiveRoot -Force | Out-Null
        New-Item -ItemType Directory -Path (Join-Path $archiveRoot "001-existing-change") | Out-Null
        New-Item -ItemType Directory -Path $activeDir -Force | Out-Null
        New-Item -ItemType Directory -Path $dateArchiveDir -Force | Out-Null
        New-Item -ItemType Directory -Path (Join-Path $dateArchiveDir "specs\capability") -Force | Out-Null

        New-Item -ItemType File -Path (Join-Path $activeDir ".openspec.yaml") | Out-Null
        New-Item -ItemType File -Path (Join-Path $activeDir "proposal.md") | Out-Null
        New-Item -ItemType File -Path (Join-Path $activeDir "tasks.md") | Out-Null

        New-Item -ItemType File -Path (Join-Path $dateArchiveDir ".openspec.yaml") | Out-Null
        New-Item -ItemType File -Path (Join-Path $dateArchiveDir "proposal.md") | Out-Null
        New-Item -ItemType File -Path (Join-Path $dateArchiveDir "tasks.md") | Out-Null
        New-Item -ItemType File -Path (Join-Path $dateArchiveDir "specs\capability\spec.md") | Out-Null

        Mock Invoke-OpenSpecArchiveCommand {}

        Archive-OneChange -Change "sample-change" -ChangesRoot $changesRoot -ArchiveRoot $archiveRoot -RepoRoot $repoRoot

        Test-Path $activeDir | Should Be $false
        Test-Path (Join-Path $archiveRoot "002-sample-change") | Should Be $true
        Assert-MockCalled Invoke-OpenSpecArchiveCommand -Times 1 -Exactly
    }
}
