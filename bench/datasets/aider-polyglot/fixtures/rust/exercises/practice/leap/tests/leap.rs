use leap::is_leap_year;

#[test]
fn year_divisible_by_4_is_leap() {
    assert!(is_leap_year(1996));
}
