# Check PRs in fork repository
$repo = "meimingqi222/crush"
$apiBase = "https://api.github.com/repos/$repo"

# Get all open PRs
$prs = Invoke-RestMethod -Uri "$apiBase/pulls?state=open" -Headers @{
    "Accept" = "application/vnd.github.v3+json"
    "User-Agent" = "PowerShell"
}

Write-Host "Found $($prs.Count) open PRs`n"

foreach ($pr in $prs) {
    Write-Host "PR #$($pr.number): $($pr.title)"
    Write-Host "  Author: $($pr.user.login)"
    Write-Host "  URL: $($pr.html_url)"
    Write-Host "  Created: $($pr.created_at)"
    
    # Get reviews for this PR
    $reviews = Invoke-RestMethod -Uri "$apiBase/pulls/$($pr.number)/reviews" -Headers @{
        "Accept" = "application/vnd.github.v3+json"
        "User-Agent" = "PowerShell"
    }
    
    if ($reviews.Count -gt 0) {
        Write-Host "  Reviews:"
        foreach ($review in $reviews) {
            Write-Host "    - $($review.user.login): $($review.state) ($($review.submitted_at))"
            if ($review.body) {
                Write-Host "      Body: $($review.body.Substring(0, [Math]::Min(200, $review.body.Length)))..."
            }
        }
    }
    
    # Get comments on this PR
    $comments = Invoke-RestMethod -Uri "$apiBase/pulls/$($pr.number)/comments" -Headers @{
        "Accept" = "application/vnd.github.v3+json"
        "User-Agent" = "PowerShell"
    }
    
    if ($comments.Count -gt 0) {
        Write-Host "  Comments: $($comments.Count)"
        foreach ($comment in $comments) {
            Write-Host "    - $($comment.user.login): $($comment.created_at)"
            if ($comment.body) {
                $preview = $comment.body.Substring(0, [Math]::Min(300, $comment.body.Length))
                Write-Host "      Body: $preview..."
            }
        }
    }
    
    Write-Host ""
}
